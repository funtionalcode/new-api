package service

import (
	"context"
	"errors"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode"

	"github.com/QuantumNous/new-api/common"
)

const cliproxyMetaQuotaURL = "https://api.meta.ai/muse-code/key"

type CliproxyMetaQuota struct {
	Plan               string                  `json:"plan,omitempty"`
	SubscriptionActive *bool                   `json:"subscription_active,omitempty"`
	Window             CliproxyMetaQuotaWindow `json:"window"`
	Weekly             CliproxyMetaQuotaWindow `json:"weekly"`
}

type CliproxyMetaQuotaWindow struct {
	UsedPercent     *float64 `json:"used_percent,omitempty"`
	ResetAt         *int64   `json:"reset_at,omitempty"`
	DurationMinutes *float64 `json:"duration_minutes,omitempty"`
}

func (client *CliproxyAPIClient) RefreshMetaQuota(ctx context.Context, authIndex string) (*CliproxyMetaQuota, error) {
	ctx, cancel := context.WithTimeout(ctx, cliproxyAPITimeout)
	defer cancel()
	files, err := client.ListAuthFiles(ctx)
	if err != nil {
		return nil, common.NewMessage("Failed to load the Meta credential. Refresh the credential list and try again.")
	}
	var name string
	for _, file := range files {
		if file.AuthIndex != authIndex {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(file.Provider), "meta") {
			break
		}
		name = file.Name
		break
	}
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\") {
		return nil, common.NewMessage("Failed to load the Meta credential. Refresh the credential list and try again.")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+cliproxyAPIAuthFilesPath+"/download?"+url.Values{"name": {name}}.Encode(), nil)
	if err != nil {
		return nil, common.NewMessage("Failed to download the Meta credential.")
	}
	client.setAuthHeader(req)
	downloadClient := *client.client
	downloadClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := downloadClient.Do(req)
	if err != nil {
		return nil, common.NewMessage("Failed to download the Meta credential.")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, common.NewMessage("Failed to download the Meta credential.")
	}
	var credential struct {
		Type     string `json:"type"`
		DCAToken string `json:"dca_token"`
	}
	const maxCredentialSize = 1024 * 1024
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxCredentialSize+1))
	if err != nil || len(raw) > maxCredentialSize || common.Unmarshal(raw, &credential) != nil {
		return nil, common.NewMessage("Failed to download the Meta credential.")
	}
	if !strings.EqualFold(strings.TrimSpace(credential.Type), "meta") {
		return nil, common.NewMessage("Failed to load the Meta credential. Refresh the credential list and try again.")
	}
	dcaToken := strings.TrimSpace(credential.DCAToken)
	token, hasPrefix := strings.CutPrefix(dcaToken, "dca:")
	if !hasPrefix || token == "" || strings.ContainsFunc(dcaToken, unicode.IsSpace) {
		return nil, common.NewMessage("Meta credential is missing a valid DCA token. Sign in to Meta again.")
	}
	// Meta 的额度接口只接受 DCA token；CPA 的 $TOKEN$ 会替换成模型 API Key。
	result, err := client.CallAPI(ctx, CliproxyAPICallRequest{
		AuthIndex: authIndex,
		Method:    http.MethodPost,
		URL:       cliproxyMetaQuotaURL,
		Header: map[string]string{
			"Authorization": "Bearer " + dcaToken,
			"Accept":        "application/json",
			"Content-Type":  "application/json",
			"x-api-version": "1.0.0",
			"User-Agent":    "muse-code/1.0.2",
		},
		Data: "{}",
	})
	if err != nil {
		var message *common.Message
		if errors.As(err, &message) {
			return nil, err
		}
		return nil, common.NewMessage("Meta quota request failed. Try again later.")
	}
	if result.Status != http.StatusOK || result.Body == nil || result.Body["error"] != nil {
		return nil, common.NewMessage("Invalid Meta quota response.")
	}
	quota := &CliproxyMetaQuota{}
	quota.Plan, _ = result.Body["subs_tier_name"].(string)
	if active, ok := result.Body["is_subs_active"].(bool); ok {
		quota.SubscriptionActive = &active
	}
	usage, _ := result.Body["subs_usage"].(map[string]any)
	if strings.TrimSpace(quota.Plan) == "" {
		quota.Plan, _ = usage["tier"].(string)
	}
	quota.Plan = strings.TrimSpace(quota.Plan)
	quota.Window = parseCliproxyMetaQuotaWindow(usage["window"])
	quota.Weekly = parseCliproxyMetaQuotaWindow(usage["weekly"])
	return quota, nil
}

func parseCliproxyMetaQuotaWindow(value any) CliproxyMetaQuotaWindow {
	window, _ := value.(map[string]any)
	quota := CliproxyMetaQuotaWindow{
		UsedPercent:     cliproxyMetaNumber(window["used_percent"]),
		DurationMinutes: cliproxyMetaNumber(window["window_duration_mins"]),
	}
	if quota.UsedPercent != nil {
		*quota.UsedPercent = min(100, max(0, *quota.UsedPercent))
	}
	if quota.DurationMinutes != nil && *quota.DurationMinutes <= 0 {
		quota.DurationMinutes = nil
	}
	if reset := cliproxyMetaNumber(window["resets_at"]); reset != nil && *reset > 0 && *reset <= 253402300799 && math.Trunc(*reset) == *reset {
		seconds := int64(*reset)
		quota.ResetAt = &seconds
	}
	return quota
}

func cliproxyMetaNumber(value any) *float64 {
	var number float64
	switch value := value.(type) {
	case float64:
		number = value
	case string:
		var err error
		number, err = strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil {
			return nil
		}
	default:
		return nil
	}
	if math.IsNaN(number) || math.IsInf(number, 0) {
		return nil
	}
	return &number
}

func cliproxyMetaQuotaHTTPError(status int, body map[string]any, upstream bool) error {
	message := common.NewMessage("Meta quota request failed. Try again later.")
	if code := cliproxyMetaNumber(body["code"]); code != nil && *code == 4705001 {
		message = common.NewMessage("Meta requires a verified 18+ account. Verify your age or add a payment method, then retry.")
	} else if upstream && (status == http.StatusUnauthorized || status == http.StatusForbidden) {
		message = common.NewMessage("Meta authorization expired or was denied. Sign in to Meta again.")
	}
	return &cliproxyHTTPStatusError{message: message.Error(), statusCode: status, cause: message}
}

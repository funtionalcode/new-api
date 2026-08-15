package controller

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

const (
	cursorQuotaCurrentPeriodPath   = "/api/dashboard/get-current-period-usage"
	cursorQuotaAggregatedUsagePath = "/api/dashboard/get-aggregated-usage-events"
	cursorQuotaPlanInfoPath        = "/api/dashboard/get-plan-info"
)

type cursorQuotaBindingRequest struct {
	Name            string  `json:"name"`
	Note            string  `json:"note"`
	RequestCurl     string  `json:"request_curl"`
	UsageAmountCurl string  `json:"usage_amount_curl"`
	UsageCostCurl   string  `json:"usage_cost_curl"`
	Proxy           *string `json:"proxy"`
	Enabled         bool    `json:"enabled"`
}

type cursorQuotaCurrentPeriodResponse struct {
	BillingCycleStart string `json:"billingCycleStart"`
	BillingCycleEnd   string `json:"billingCycleEnd"`
	PlanUsage         *struct {
		TotalSpend       float64 `json:"totalSpend"`
		Limit            float64 `json:"limit"`
		Remaining        float64 `json:"remaining"`
		AutoPercentUsed  float64 `json:"autoPercentUsed"`
		APIPercentUsed   float64 `json:"apiPercentUsed"`
		TotalPercentUsed float64 `json:"totalPercentUsed"`
	} `json:"planUsage"`
	SpendLimitUsage *struct {
		IndividualLimit     float64 `json:"individualLimit"`
		IndividualRemaining float64 `json:"individualRemaining"`
	} `json:"spendLimitUsage"`
}

type cursorQuotaPlanInfoResponse struct {
	PlanInfo *struct {
		PlanName            string  `json:"planName"`
		IncludedAmountCents float64 `json:"includedAmountCents"`
		Price               string  `json:"price"`
	} `json:"planInfo"`
}

type cursorQuotaAggregatedUsageResponse struct {
	Aggregations          []cursorQuotaAggregation `json:"aggregations"`
	TotalInputTokens      cursorQuotaTokenCount    `json:"totalInputTokens"`
	TotalOutputTokens     cursorQuotaTokenCount    `json:"totalOutputTokens"`
	TotalCacheWriteTokens cursorQuotaTokenCount    `json:"totalCacheWriteTokens"`
	TotalCacheReadTokens  cursorQuotaTokenCount    `json:"totalCacheReadTokens"`
	TotalCostCents        float64                  `json:"totalCostCents"`
}

type cursorQuotaAggregation struct {
	ModelIntent      string                `json:"modelIntent"`
	InputTokens      cursorQuotaTokenCount `json:"inputTokens"`
	OutputTokens     cursorQuotaTokenCount `json:"outputTokens"`
	CacheWriteTokens cursorQuotaTokenCount `json:"cacheWriteTokens"`
	CacheReadTokens  cursorQuotaTokenCount `json:"cacheReadTokens"`
	TotalCents       float64               `json:"totalCents"`
	Tier             int                   `json:"tier"`
}

type cursorQuotaTokenCount int64

func (count *cursorQuotaTokenCount) UnmarshalJSON(data []byte) error {
	raw := strings.TrimSpace(string(data))
	if raw == "" || raw == "null" {
		*count = 0
		return nil
	}
	if len(raw) >= 2 && raw[0] == '"' && raw[len(raw)-1] == '"' {
		raw = raw[1 : len(raw)-1]
	}
	value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return fmt.Errorf("Cursor token 数量无效: %w", err)
	}
	if value < 0 {
		value = 0
	}
	*count = cursorQuotaTokenCount(value)
	return nil
}

type cursorQuotaModelUsage struct {
	Model            string  `json:"model"`
	InputTokens      int64   `json:"input_tokens"`
	OutputTokens     int64   `json:"output_tokens"`
	CacheWriteTokens int64   `json:"cache_write_tokens"`
	CacheReadTokens  int64   `json:"cache_read_tokens"`
	TotalTokens      int64   `json:"total_tokens"`
	TotalCents       float64 `json:"total_cents"`
	Tier             int     `json:"tier"`
}

type cursorQuotaUsageRefreshBody struct {
	PlanName               string
	BillingCycleStartAt    int64
	BillingCycleEndAt      int64
	PlanUsedCents          float64
	PlanLimitCents         float64
	PlanRemainingCents     float64
	PlanAPIPercent         float64
	PlanTotalPercent       float64
	OnDemandUsedCents      float64
	OnDemandLimitCents     float64
	OnDemandRemainingCents float64
	TotalInputTokens       int64
	TotalOutputTokens      int64
	TotalCacheWriteTokens  int64
	TotalCacheReadTokens   int64
	TotalCostCents         float64
	ModelUsages            []cursorQuotaModelUsage
}

func GetCursorQuotaBindings(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	query := model.CursorQuotaBindingQuery{
		Keyword: c.Query("keyword"),
		Enabled: parseOptionalBool(c.Query("enabled")),
	}
	bindings, total, err := model.GetCursorQuotaBindings(query, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	sanitizeCursorQuotaBindingsForRole(bindings, c.GetInt("role"))
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(bindings)
	common.ApiSuccess(c, pageInfo)
}

func CreateCursorQuotaBinding(c *gin.Context) {
	update, err := decodeCursorQuotaBindingRequest(c, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	binding := &model.CursorQuotaBinding{
		Name:            update.Name,
		Note:            update.Note,
		RequestCurl:     update.RequestCurl,
		UsageAmountCurl: update.UsageAmountCurl,
		UsageCostCurl:   update.UsageCostCurl,
		Proxy:           update.Proxy,
		Enabled:         update.Enabled,
	}
	if err := model.CreateCursorQuotaBinding(binding); err != nil {
		common.ApiError(c, err)
		return
	}
	binding.HasCurl = true
	binding.HasUsageAmountCurl = true
	binding.HasUsageCostCurl = true
	common.ApiSuccess(c, binding)
}

func UpdateCursorQuotaBinding(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("Cursor 额度配置 ID 无效"))
		return
	}
	update, err := decodeCursorQuotaBindingRequest(c, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	binding, err := model.UpdateCursorQuotaBinding(id, update)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, binding)
}

func DeleteCursorQuotaBinding(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("Cursor 额度配置 ID 无效"))
		return
	}
	if err := model.DeleteCursorQuotaBindingById(id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func RefreshCursorQuotaBindingUsage(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("Cursor 额度配置 ID 无效"))
		return
	}
	binding, err := model.GetCursorQuotaBindingById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !binding.Enabled {
		common.ApiError(c, fmt.Errorf("Cursor 额度配置已禁用"))
		return
	}
	usage, err := refreshCursorQuotaUsage(c.Request.Context(), binding)
	if err != nil {
		updatedBinding, updateErr := model.UpdateCursorQuotaBindingUsage(id, model.CursorQuotaUsageRefreshUpdate{LastError: err.Error()})
		if updateErr != nil {
			common.ApiError(c, fmt.Errorf("刷新额度失败: %s；保存错误失败: %w", err.Error(), updateErr))
			return
		}
		sanitizeCursorQuotaBindingForRole(updatedBinding, c.GetInt("role"))
		common.ApiSuccess(c, updatedBinding)
		return
	}
	modelUsage, err := common.Marshal(usage.ModelUsages)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	updatedBinding, err := model.UpdateCursorQuotaBindingUsage(id, model.CursorQuotaUsageRefreshUpdate{
		LastPlanName:               usage.PlanName,
		LastBillingCycleStartAt:    usage.BillingCycleStartAt,
		LastBillingCycleEndAt:      usage.BillingCycleEndAt,
		LastPlanUsedCents:          usage.PlanUsedCents,
		LastPlanLimitCents:         usage.PlanLimitCents,
		LastPlanRemainingCents:     usage.PlanRemainingCents,
		LastPlanAPIPercent:         usage.PlanAPIPercent,
		LastPlanTotalPercent:       usage.PlanTotalPercent,
		LastOnDemandUsedCents:      usage.OnDemandUsedCents,
		LastOnDemandLimitCents:     usage.OnDemandLimitCents,
		LastOnDemandRemainingCents: usage.OnDemandRemainingCents,
		LastTotalInputTokens:       usage.TotalInputTokens,
		LastTotalOutputTokens:      usage.TotalOutputTokens,
		LastTotalCacheWriteTokens:  usage.TotalCacheWriteTokens,
		LastTotalCacheReadTokens:   usage.TotalCacheReadTokens,
		LastTotalCostCents:         usage.TotalCostCents,
		LastModelUsage:             string(modelUsage),
		LastError:                  "",
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	sanitizeCursorQuotaBindingForRole(updatedBinding, c.GetInt("role"))
	common.ApiSuccess(c, updatedBinding)
}

func sanitizeCursorQuotaBindingsForRole(bindings []*model.CursorQuotaBinding, role int) {
	for _, binding := range bindings {
		sanitizeCursorQuotaBindingForRole(binding, role)
	}
}

func sanitizeCursorQuotaBindingForRole(binding *model.CursorQuotaBinding, role int) {
	if binding == nil || role >= common.RoleAdminUser {
		return
	}
	binding.RequestCurl = ""
	binding.UsageAmountCurl = ""
	binding.UsageCostCurl = ""
	binding.Proxy = ""
}

func decodeCursorQuotaBindingRequest(c *gin.Context, requireCurl bool) (model.CursorQuotaBindingUpdate, error) {
	var request cursorQuotaBindingRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		return model.CursorQuotaBindingUpdate{}, fmt.Errorf("无效的参数")
	}
	update := model.CursorQuotaBindingUpdate{
		Name:                  strings.TrimSpace(request.Name),
		Note:                  strings.TrimSpace(request.Note),
		RequestCurl:           strings.TrimSpace(request.RequestCurl),
		UpdateCurl:            strings.TrimSpace(request.RequestCurl) != "",
		UsageAmountCurl:       strings.TrimSpace(request.UsageAmountCurl),
		UpdateUsageAmountCurl: strings.TrimSpace(request.UsageAmountCurl) != "",
		UsageCostCurl:         strings.TrimSpace(request.UsageCostCurl),
		UpdateUsageCostCurl:   strings.TrimSpace(request.UsageCostCurl) != "",
		UpdateProxy:           request.Proxy != nil,
		Enabled:               request.Enabled,
	}
	if request.Proxy != nil {
		update.Proxy = strings.TrimSpace(*request.Proxy)
	}
	if err := model.ValidateCursorQuotaBindingUpdate(update, requireCurl); err != nil {
		return model.CursorQuotaBindingUpdate{}, err
	}
	return update, nil
}

func refreshCursorQuotaUsage(ctx context.Context, binding *model.CursorQuotaBinding) (cursorQuotaUsageRefreshBody, error) {
	periodBody, err := executeCursorQuotaCurl(ctx, binding.RequestCurl, binding.Proxy, cursorQuotaCurrentPeriodPath, nil)
	if err != nil {
		return cursorQuotaUsageRefreshBody{}, err
	}
	_, startAt, _, err := extractCursorQuotaPeriod(periodBody)
	if err != nil {
		return cursorQuotaUsageRefreshBody{}, err
	}

	aggregatedBody, err := executeCursorQuotaCurl(
		ctx,
		binding.UsageAmountCurl,
		binding.Proxy,
		cursorQuotaAggregatedUsagePath,
		func(requestConfig *quotaCurlRequest) error {
			body, err := updateCursorAggregatedUsageRequestBody(requestConfig.Body, startAt)
			if err != nil {
				return err
			}
			requestConfig.Body = body
			return nil
		},
	)
	if err != nil {
		return cursorQuotaUsageRefreshBody{}, err
	}
	planBody, err := executeCursorQuotaCurl(ctx, binding.UsageCostCurl, binding.Proxy, cursorQuotaPlanInfoPath, nil)
	if err != nil {
		return cursorQuotaUsageRefreshBody{}, err
	}

	return extractCursorQuotaUsage(periodBody, aggregatedBody, planBody)
}

func executeCursorQuotaCurl(
	ctx context.Context,
	rawCurl string,
	bindingProxy string,
	expectedPath string,
	mutate func(*quotaCurlRequest) error,
) ([]byte, error) {
	requestConfig, err := buildCursorQuotaCurlRequest(rawCurl)
	if err != nil {
		return nil, err
	}
	if err := validateCursorQuotaEndpoint(requestConfig.URL, expectedPath); err != nil {
		return nil, err
	}
	if requestConfig.Method != http.MethodPost {
		return nil, fmt.Errorf("Cursor 额度 curl 必须使用 POST 请求")
	}
	if mutate != nil {
		if err := mutate(&requestConfig); err != nil {
			return nil, err
		}
	}
	request, err := http.NewRequestWithContext(ctx, requestConfig.Method, requestConfig.URL, strings.NewReader(requestConfig.Body))
	if err != nil {
		return nil, err
	}
	for key, value := range requestConfig.Headers {
		if strings.EqualFold(key, "host") {
			request.Host = value
			continue
		}
		if shouldSkipQuotaCurlHeader(key) {
			continue
		}
		request.Header.Set(key, value)
	}
	if request.Header.Get("User-Agent") == "" {
		request.Header.Set("User-Agent", "Mozilla/5.0")
	}
	if request.Header.Get("Content-Type") == "" {
		request.Header.Set("Content-Type", "application/json")
	}
	proxyURL := resolveQuotaProxy(bindingProxy, requestConfig.Proxy)
	client, err := quotaHTTPClient(proxyURL)
	if err != nil {
		if strings.TrimSpace(proxyURL) != "" {
			return nil, fmt.Errorf("创建代理客户端 %s 失败: %w", quotaProxyLabel(proxyURL), err)
		}
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, quotaHTTPRequestError(proxyURL, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("Cursor 返回 HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, nil
}

func buildCursorQuotaCurlRequest(rawCurl string) (quotaCurlRequest, error) {
	return parseQuotaCurlRequest(rawCurl)
}

func validateCursorQuotaEndpoint(rawURL string, expectedPath string) error {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	if !strings.EqualFold(parsedURL.Scheme, "https") || !strings.EqualFold(parsedURL.Hostname(), "cursor.com") {
		return fmt.Errorf("Cursor 额度 curl 必须请求 https://cursor.com")
	}
	if parsedURL.Port() != "" && parsedURL.Port() != "443" {
		return fmt.Errorf("Cursor 额度 curl 端口无效")
	}
	if parsedURL.Path != expectedPath {
		return fmt.Errorf("Cursor 额度 curl 接口不匹配，期望 %s", expectedPath)
	}
	return nil
}

func updateCursorAggregatedUsageRequestBody(body string, billingCycleStartAt int64) (string, error) {
	if billingCycleStartAt <= 0 || billingCycleStartAt > math.MaxInt64/1000 {
		return "", fmt.Errorf("Cursor 账期开始时间无效")
	}
	requestBody := map[string]any{}
	if strings.TrimSpace(body) != "" {
		if err := common.UnmarshalJsonStr(body, &requestBody); err != nil {
			return "", fmt.Errorf("Cursor 聚合用量 curl 请求体无效: %w", err)
		}
	}
	requestBody["startDate"] = billingCycleStartAt * 1000
	updatedBody, err := common.Marshal(requestBody)
	if err != nil {
		return "", err
	}
	return string(updatedBody), nil
}

func extractCursorQuotaUsage(periodBody []byte, aggregatedBody []byte, planBody []byte) (cursorQuotaUsageRefreshBody, error) {
	period, startAt, endAt, err := extractCursorQuotaPeriod(periodBody)
	if err != nil {
		return cursorQuotaUsageRefreshBody{}, err
	}
	var aggregated cursorQuotaAggregatedUsageResponse
	if err := common.Unmarshal(aggregatedBody, &aggregated); err != nil {
		return cursorQuotaUsageRefreshBody{}, fmt.Errorf("解析 Cursor 聚合用量失败: %w", err)
	}
	var plan cursorQuotaPlanInfoResponse
	if err := common.Unmarshal(planBody, &plan); err != nil {
		return cursorQuotaUsageRefreshBody{}, fmt.Errorf("解析 Cursor 套餐信息失败: %w", err)
	}
	if plan.PlanInfo == nil {
		return cursorQuotaUsageRefreshBody{}, fmt.Errorf("Cursor 套餐信息响应缺少 planInfo")
	}

	modelUsages := make([]cursorQuotaModelUsage, 0, len(aggregated.Aggregations))
	for _, item := range aggregated.Aggregations {
		inputTokens := int64(item.InputTokens)
		outputTokens := int64(item.OutputTokens)
		cacheWriteTokens := int64(item.CacheWriteTokens)
		cacheReadTokens := int64(item.CacheReadTokens)
		modelUsages = append(modelUsages, cursorQuotaModelUsage{
			Model:            strings.TrimSpace(item.ModelIntent),
			InputTokens:      inputTokens,
			OutputTokens:     outputTokens,
			CacheWriteTokens: cacheWriteTokens,
			CacheReadTokens:  cacheReadTokens,
			TotalTokens:      cursorQuotaSafeTokenSum(inputTokens, outputTokens, cacheWriteTokens, cacheReadTokens),
			TotalCents:       normalizeCursorQuotaNumber(item.TotalCents),
			Tier:             item.Tier,
		})
	}

	planLimit := normalizeCursorQuotaNumber(period.PlanUsage.Limit)
	if planLimit == 0 {
		planLimit = normalizeCursorQuotaNumber(plan.PlanInfo.IncludedAmountCents)
	}
	onDemandLimit := 0.0
	onDemandRemaining := 0.0
	if period.SpendLimitUsage != nil {
		onDemandLimit = normalizeCursorQuotaNumber(period.SpendLimitUsage.IndividualLimit)
		onDemandRemaining = normalizeCursorQuotaNumber(period.SpendLimitUsage.IndividualRemaining)
	}
	onDemandUsed := math.Max(0, onDemandLimit-onDemandRemaining)

	return cursorQuotaUsageRefreshBody{
		PlanName:               strings.TrimSpace(plan.PlanInfo.PlanName),
		BillingCycleStartAt:    startAt,
		BillingCycleEndAt:      endAt,
		PlanUsedCents:          normalizeCursorQuotaNumber(period.PlanUsage.TotalSpend),
		PlanLimitCents:         planLimit,
		PlanRemainingCents:     normalizeCursorQuotaNumber(period.PlanUsage.Remaining),
		PlanAPIPercent:         normalizeCursorQuotaNumber(period.PlanUsage.APIPercentUsed),
		PlanTotalPercent:       normalizeCursorQuotaNumber(period.PlanUsage.TotalPercentUsed),
		OnDemandUsedCents:      onDemandUsed,
		OnDemandLimitCents:     onDemandLimit,
		OnDemandRemainingCents: onDemandRemaining,
		TotalInputTokens:       int64(aggregated.TotalInputTokens),
		TotalOutputTokens:      int64(aggregated.TotalOutputTokens),
		TotalCacheWriteTokens:  int64(aggregated.TotalCacheWriteTokens),
		TotalCacheReadTokens:   int64(aggregated.TotalCacheReadTokens),
		TotalCostCents:         normalizeCursorQuotaNumber(aggregated.TotalCostCents),
		ModelUsages:            modelUsages,
	}, nil
}

func extractCursorQuotaPeriod(body []byte) (cursorQuotaCurrentPeriodResponse, int64, int64, error) {
	var period cursorQuotaCurrentPeriodResponse
	if err := common.Unmarshal(body, &period); err != nil {
		return cursorQuotaCurrentPeriodResponse{}, 0, 0, fmt.Errorf("解析 Cursor 当前周期用量失败: %w", err)
	}
	if period.PlanUsage == nil {
		return cursorQuotaCurrentPeriodResponse{}, 0, 0, fmt.Errorf("Cursor 当前周期用量响应缺少 planUsage")
	}
	startAt, err := parseCursorQuotaTimestamp(period.BillingCycleStart)
	if err != nil {
		return cursorQuotaCurrentPeriodResponse{}, 0, 0, fmt.Errorf("Cursor 账期开始时间无效: %w", err)
	}
	endAt, err := parseCursorQuotaTimestamp(period.BillingCycleEnd)
	if err != nil {
		return cursorQuotaCurrentPeriodResponse{}, 0, 0, fmt.Errorf("Cursor 账期结束时间无效: %w", err)
	}
	return period, startAt, endAt, nil
}

func parseCursorQuotaTimestamp(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("时间为空")
	}
	isDigits := true
	for _, char := range value {
		if char < '0' || char > '9' {
			isDigits = false
			break
		}
	}
	if isDigits {
		timestamp, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return 0, err
		}
		if timestamp <= 0 {
			return 0, fmt.Errorf("时间戳必须为正数")
		}
		if timestamp >= 1_000_000_000_000 {
			return timestamp / 1000, nil
		}
		return timestamp, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return 0, err
	}
	return parsed.Unix(), nil
}

func normalizeCursorQuotaNumber(value float64) float64 {
	if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return value
}

func cursorQuotaSafeTokenSum(values ...int64) int64 {
	total := int64(0)
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if value > math.MaxInt64-total {
			return math.MaxInt64
		}
		total += value
	}
	return total
}

package controller

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

const maxKimiQuotaValue = int64(1<<63 - 1)

type kimiQuotaBindingRequest struct {
	Name        string  `json:"name"`
	Note        string  `json:"note"`
	RequestCurl string  `json:"request_curl"`
	Proxy       *string `json:"proxy"`
	Enabled     bool    `json:"enabled"`
}

type kimiQuotaAccountInfoResponse struct {
	Code    int                      `json:"code"`
	Data    kimiQuotaAccountInfoData `json:"data"`
	Message string                   `json:"message"`
}

type kimiQuotaAccountInfoData struct {
	CurrentQuota            int64 `json:"cur"`
	VoucherCurrentQuota     int64 `json:"voucher_cur"`
	AccumulatedQuota        int64 `json:"acc"`
	VoucherAccumulatedQuota int64 `json:"voucher_acc"`
	VoucherExpiredQuota     int64 `json:"voucher_expired"`
	RechargeBonusPercent    int   `json:"recharge_bonus_percent"`
	UsedQuota               int64 `json:"use"`
}

type kimiQuotaUsageRefreshBody struct {
	CurrentQuota            int64
	VoucherCurrentQuota     int64
	AccumulatedQuota        int64
	VoucherAccumulatedQuota int64
	VoucherExpiredQuota     int64
	RechargeBonusPercent    int
	UsedQuota               int64
	RemainingQuota          int64
	TotalQuota              int64
	RemainingPercent        int
}

func GetKimiQuotaBindings(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	query := model.KimiQuotaBindingQuery{
		Keyword: c.Query("keyword"),
		Enabled: parseOptionalBool(c.Query("enabled")),
	}
	bindings, total, err := model.GetKimiQuotaBindings(query, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	sanitizeKimiQuotaBindingsForRole(bindings, c.GetInt("role"))
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(bindings)
	common.ApiSuccess(c, pageInfo)
}

func CreateKimiQuotaBinding(c *gin.Context) {
	update, err := decodeKimiQuotaBindingRequest(c, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	binding := &model.KimiQuotaBinding{
		Name:        update.Name,
		Note:        update.Note,
		RequestCurl: update.RequestCurl,
		Proxy:       update.Proxy,
		Enabled:     update.Enabled,
	}
	if err := model.CreateKimiQuotaBinding(binding); err != nil {
		common.ApiError(c, err)
		return
	}
	binding.HasCurl = strings.TrimSpace(binding.RequestCurl) != ""
	common.ApiSuccess(c, binding)
}

func UpdateKimiQuotaBinding(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("Kimi 额度配置 ID 无效"))
		return
	}
	update, err := decodeKimiQuotaBindingRequest(c, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	binding, err := model.UpdateKimiQuotaBinding(id, update)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, binding)
}

func DeleteKimiQuotaBinding(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("Kimi 额度配置 ID 无效"))
		return
	}
	if err := model.DeleteKimiQuotaBindingById(id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func RefreshKimiQuotaBindingUsage(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("Kimi 额度配置 ID 无效"))
		return
	}
	binding, err := model.GetKimiQuotaBindingById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !binding.Enabled {
		common.ApiError(c, fmt.Errorf("Kimi 额度配置已禁用"))
		return
	}
	usage, err := refreshKimiQuotaUsage(c.Request.Context(), binding)
	if err != nil {
		updatedBinding, updateErr := model.UpdateKimiQuotaBindingUsage(id, model.KimiQuotaUsageRefreshUpdate{LastError: err.Error()})
		if updateErr != nil {
			common.ApiError(c, fmt.Errorf("刷新额度失败: %s；保存错误失败: %w", err.Error(), updateErr))
			return
		}
		sanitizeKimiQuotaBindingForRole(updatedBinding, c.GetInt("role"))
		common.ApiSuccess(c, updatedBinding)
		return
	}
	updatedBinding, err := model.UpdateKimiQuotaBindingUsage(id, model.KimiQuotaUsageRefreshUpdate{
		LastCurrentQuota:            usage.CurrentQuota,
		LastVoucherCurrentQuota:     usage.VoucherCurrentQuota,
		LastAccumulatedQuota:        usage.AccumulatedQuota,
		LastVoucherAccumulatedQuota: usage.VoucherAccumulatedQuota,
		LastVoucherExpiredQuota:     usage.VoucherExpiredQuota,
		LastRechargeBonusPercent:    usage.RechargeBonusPercent,
		LastUsedQuota:               usage.UsedQuota,
		LastRemainingQuota:          usage.RemainingQuota,
		LastTotalQuota:              usage.TotalQuota,
		LastRemainingPercent:        usage.RemainingPercent,
		LastError:                   "",
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	sanitizeKimiQuotaBindingForRole(updatedBinding, c.GetInt("role"))
	common.ApiSuccess(c, updatedBinding)
}

func sanitizeKimiQuotaBindingsForRole(bindings []*model.KimiQuotaBinding, role int) {
	for _, binding := range bindings {
		sanitizeKimiQuotaBindingForRole(binding, role)
	}
}

func sanitizeKimiQuotaBindingForRole(binding *model.KimiQuotaBinding, role int) {
	if binding == nil || role >= common.RoleAdminUser {
		return
	}
	binding.RequestCurl = ""
	binding.Proxy = ""
}

func decodeKimiQuotaBindingRequest(c *gin.Context, requireCurl bool) (model.KimiQuotaBindingUpdate, error) {
	var request kimiQuotaBindingRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		return model.KimiQuotaBindingUpdate{}, fmt.Errorf("无效的参数")
	}
	update := model.KimiQuotaBindingUpdate{
		Name:        strings.TrimSpace(request.Name),
		Note:        strings.TrimSpace(request.Note),
		RequestCurl: strings.TrimSpace(request.RequestCurl),
		UpdateCurl:  strings.TrimSpace(request.RequestCurl) != "",
		UpdateProxy: request.Proxy != nil,
		Enabled:     request.Enabled,
	}
	if request.Proxy != nil {
		update.Proxy = strings.TrimSpace(*request.Proxy)
	}
	if err := model.ValidateKimiQuotaBindingUpdate(update, requireCurl); err != nil {
		return model.KimiQuotaBindingUpdate{}, err
	}
	return update, nil
}

func refreshKimiQuotaUsage(ctx context.Context, binding *model.KimiQuotaBinding) (kimiQuotaUsageRefreshBody, error) {
	requestConfig, err := buildKimiQuotaCurlRequest(binding.RequestCurl)
	if err != nil {
		return kimiQuotaUsageRefreshBody{}, err
	}
	request, err := http.NewRequestWithContext(ctx, requestConfig.Method, requestConfig.URL, nil)
	if err != nil {
		return kimiQuotaUsageRefreshBody{}, err
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
	proxyURL := resolveQuotaProxy(binding.Proxy, requestConfig.Proxy)
	client, err := quotaHTTPClient(proxyURL)
	if err != nil {
		if strings.TrimSpace(proxyURL) != "" {
			return kimiQuotaUsageRefreshBody{}, fmt.Errorf("创建代理客户端 %s 失败: %w", quotaProxyLabel(proxyURL), err)
		}
		return kimiQuotaUsageRefreshBody{}, err
	}
	response, err := client.Do(request)
	if err != nil {
		return kimiQuotaUsageRefreshBody{}, quotaHTTPRequestError(proxyURL, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return kimiQuotaUsageRefreshBody{}, err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return kimiQuotaUsageRefreshBody{}, fmt.Errorf("Kimi 返回 HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	return extractKimiQuotaUsage(body)
}

func buildKimiQuotaCurlRequest(rawCurl string) (quotaCurlRequest, error) {
	return parseQuotaCurlRequest(rawCurl)
}

func extractKimiQuotaUsage(body []byte) (kimiQuotaUsageRefreshBody, error) {
	var response kimiQuotaAccountInfoResponse
	if err := common.Unmarshal(body, &response); err != nil {
		return kimiQuotaUsageRefreshBody{}, err
	}
	if response.Code != 0 {
		return kimiQuotaUsageRefreshBody{}, fmt.Errorf("Kimi 返回错误: %s", firstNonEmpty(response.Message, strconv.Itoa(response.Code)))
	}
	data := response.Data
	currentQuota := nonNegativeKimiQuota(data.CurrentQuota)
	voucherCurrentQuota := nonNegativeKimiQuota(data.VoucherCurrentQuota)
	accumulatedQuota := nonNegativeKimiQuota(data.AccumulatedQuota)
	voucherAccumulatedQuota := nonNegativeKimiQuota(data.VoucherAccumulatedQuota)
	usedQuota := nonNegativeKimiQuota(data.UsedQuota)
	remainingQuota := sumKimiQuota(currentQuota, voucherCurrentQuota)
	totalQuota := sumKimiQuota(accumulatedQuota, voucherAccumulatedQuota)
	if totalQuota < remainingQuota {
		totalQuota = remainingQuota
	}
	if usedQuota > 0 && totalQuota < sumKimiQuota(remainingQuota, usedQuota) {
		totalQuota = sumKimiQuota(remainingQuota, usedQuota)
	}
	return kimiQuotaUsageRefreshBody{
		CurrentQuota:            currentQuota,
		VoucherCurrentQuota:     voucherCurrentQuota,
		AccumulatedQuota:        accumulatedQuota,
		VoucherAccumulatedQuota: voucherAccumulatedQuota,
		VoucherExpiredQuota:     nonNegativeKimiQuota(data.VoucherExpiredQuota),
		RechargeBonusPercent:    data.RechargeBonusPercent,
		UsedQuota:               usedQuota,
		RemainingQuota:          remainingQuota,
		TotalQuota:              totalQuota,
		RemainingPercent:        kimiQuotaRemainingPercent(remainingQuota, totalQuota),
	}, nil
}

func nonNegativeKimiQuota(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

func sumKimiQuota(values ...int64) int64 {
	var total int64
	for _, rawValue := range values {
		value := nonNegativeKimiQuota(rawValue)
		if value > maxKimiQuotaValue-total {
			return maxKimiQuotaValue
		}
		total += value
	}
	return total
}

func kimiQuotaRemainingPercent(remainingQuota int64, totalQuota int64) int {
	if totalQuota <= 0 {
		return 0
	}
	percent := int(math.Round(float64(remainingQuota) * 100 / float64(totalQuota)))
	if percent < 0 {
		return 0
	}
	if percent > 100 {
		return 100
	}
	return percent
}

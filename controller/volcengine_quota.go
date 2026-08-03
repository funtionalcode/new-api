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

type volcengineQuotaBindingRequest struct {
	Name        string  `json:"name"`
	Note        string  `json:"note"`
	RequestCurl string  `json:"request_curl"`
	Proxy       *string `json:"proxy"`
	Enabled     bool    `json:"enabled"`
}

type volcengineQuotaResponse struct {
	ResponseMetadata struct {
		Error *struct {
			Code    string `json:"Code"`
			Message string `json:"Message"`
		} `json:"Error"`
	} `json:"ResponseMetadata"`
	Result *struct {
		PlanType    string                `json:"PlanType"`
		FiveHourAFP volcengineQuotaWindow `json:"AFPFiveHour"`
		DailyAFP    volcengineQuotaWindow `json:"AFPDaily"`
		WeeklyAFP   volcengineQuotaWindow `json:"AFPWeekly"`
		MonthlyAFP  volcengineQuotaWindow `json:"AFPMonthly"`
	} `json:"Result"`
}

type volcengineQuotaWindow struct {
	Quota         int64   `json:"Quota"`
	UsedAFP       float64 `json:"Used"`
	SubscribeTime int64   `json:"SubscribeTime"`
	ResetTime     int64   `json:"ResetTime"`
}

type volcengineQuotaUsageRefreshBody struct {
	PlanType string
	FiveHour volcengineQuotaWindow
	Daily    volcengineQuotaWindow
	Weekly   volcengineQuotaWindow
	Monthly  volcengineQuotaWindow
}

func GetVolcengineQuotaBindings(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	query := model.VolcengineQuotaBindingQuery{
		Keyword: c.Query("keyword"),
		Enabled: parseOptionalBool(c.Query("enabled")),
	}
	bindings, total, err := model.GetVolcengineQuotaBindings(query, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	sanitizeVolcengineQuotaBindingsForRole(bindings, c.GetInt("role"))
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(bindings)
	common.ApiSuccess(c, pageInfo)
}

func CreateVolcengineQuotaBinding(c *gin.Context) {
	update, err := decodeVolcengineQuotaBindingRequest(c, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	binding := &model.VolcengineQuotaBinding{
		Name:        update.Name,
		Note:        update.Note,
		RequestCurl: update.RequestCurl,
		Proxy:       update.Proxy,
		Enabled:     update.Enabled,
	}
	if err := model.CreateVolcengineQuotaBinding(binding); err != nil {
		common.ApiError(c, err)
		return
	}
	binding.HasCurl = strings.TrimSpace(binding.RequestCurl) != ""
	common.ApiSuccess(c, binding)
}

func UpdateVolcengineQuotaBinding(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("火山额度配置 ID 无效"))
		return
	}
	update, err := decodeVolcengineQuotaBindingRequest(c, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	binding, err := model.UpdateVolcengineQuotaBinding(id, update)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, binding)
}

func DeleteVolcengineQuotaBinding(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("火山额度配置 ID 无效"))
		return
	}
	if err := model.DeleteVolcengineQuotaBindingById(id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func RefreshVolcengineQuotaBindingUsage(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("火山额度配置 ID 无效"))
		return
	}
	binding, err := model.GetVolcengineQuotaBindingById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !binding.Enabled {
		common.ApiError(c, fmt.Errorf("火山额度配置已禁用"))
		return
	}
	usage, err := refreshVolcengineQuotaUsage(c.Request.Context(), binding)
	if err != nil {
		updatedBinding, updateErr := model.UpdateVolcengineQuotaBindingUsage(id, model.VolcengineQuotaUsageRefreshUpdate{LastError: err.Error()})
		if updateErr != nil {
			common.ApiError(c, fmt.Errorf("刷新额度失败: %s；保存错误失败: %w", err.Error(), updateErr))
			return
		}
		sanitizeVolcengineQuotaBindingForRole(updatedBinding, c.GetInt("role"))
		common.ApiSuccess(c, updatedBinding)
		return
	}
	updatedBinding, err := model.UpdateVolcengineQuotaBindingUsage(id, model.VolcengineQuotaUsageRefreshUpdate{
		LastPlanType:            usage.PlanType,
		LastFiveHourQuota:       usage.FiveHour.Quota,
		LastFiveHourUsedAFP:     usage.FiveHour.UsedAFP,
		LastFiveHourSubscribeAt: usage.FiveHour.SubscribeTime,
		LastFiveHourResetAt:     usage.FiveHour.ResetTime,
		LastDailyQuota:          usage.Daily.Quota,
		LastDailyUsedAFP:        usage.Daily.UsedAFP,
		LastDailySubscribeAt:    usage.Daily.SubscribeTime,
		LastDailyResetAt:        usage.Daily.ResetTime,
		LastWeeklyQuota:         usage.Weekly.Quota,
		LastWeeklyUsedAFP:       usage.Weekly.UsedAFP,
		LastWeeklySubscribeAt:   usage.Weekly.SubscribeTime,
		LastWeeklyResetAt:       usage.Weekly.ResetTime,
		LastMonthlyQuota:        usage.Monthly.Quota,
		LastMonthlyUsedAFP:      usage.Monthly.UsedAFP,
		LastMonthlySubscribeAt:  usage.Monthly.SubscribeTime,
		LastMonthlyResetAt:      usage.Monthly.ResetTime,
		LastError:               "",
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	sanitizeVolcengineQuotaBindingForRole(updatedBinding, c.GetInt("role"))
	common.ApiSuccess(c, updatedBinding)
}

func sanitizeVolcengineQuotaBindingsForRole(bindings []*model.VolcengineQuotaBinding, role int) {
	for _, binding := range bindings {
		sanitizeVolcengineQuotaBindingForRole(binding, role)
	}
}

func sanitizeVolcengineQuotaBindingForRole(binding *model.VolcengineQuotaBinding, role int) {
	if binding == nil || role >= common.RoleAdminUser {
		return
	}
	binding.RequestCurl = ""
	binding.Proxy = ""
}

func decodeVolcengineQuotaBindingRequest(c *gin.Context, requireCurl bool) (model.VolcengineQuotaBindingUpdate, error) {
	var request volcengineQuotaBindingRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		return model.VolcengineQuotaBindingUpdate{}, fmt.Errorf("无效的参数")
	}
	update := model.VolcengineQuotaBindingUpdate{
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
	if err := model.ValidateVolcengineQuotaBindingUpdate(update, requireCurl); err != nil {
		return model.VolcengineQuotaBindingUpdate{}, err
	}
	return update, nil
}

func refreshVolcengineQuotaUsage(ctx context.Context, binding *model.VolcengineQuotaBinding) (volcengineQuotaUsageRefreshBody, error) {
	requestConfig, err := buildVolcengineQuotaCurlRequest(binding.RequestCurl)
	if err != nil {
		return volcengineQuotaUsageRefreshBody{}, err
	}
	request, err := http.NewRequestWithContext(ctx, requestConfig.Method, requestConfig.URL, strings.NewReader(requestConfig.Body))
	if err != nil {
		return volcengineQuotaUsageRefreshBody{}, err
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
			return volcengineQuotaUsageRefreshBody{}, fmt.Errorf("创建代理客户端 %s 失败: %w", quotaProxyLabel(proxyURL), err)
		}
		return volcengineQuotaUsageRefreshBody{}, err
	}
	response, err := client.Do(request)
	if err != nil {
		return volcengineQuotaUsageRefreshBody{}, quotaHTTPRequestError(proxyURL, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return volcengineQuotaUsageRefreshBody{}, err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return volcengineQuotaUsageRefreshBody{}, fmt.Errorf("火山引擎返回 HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	return extractVolcengineQuotaUsage(body)
}

func buildVolcengineQuotaCurlRequest(rawCurl string) (quotaCurlRequest, error) {
	return parseQuotaCurlRequest(rawCurl)
}

func extractVolcengineQuotaUsage(body []byte) (volcengineQuotaUsageRefreshBody, error) {
	var response volcengineQuotaResponse
	if err := common.Unmarshal(body, &response); err != nil {
		return volcengineQuotaUsageRefreshBody{}, err
	}
	if response.ResponseMetadata.Error != nil {
		providerError := response.ResponseMetadata.Error
		errorCode := strings.TrimSpace(providerError.Code)
		errorMessage := strings.TrimSpace(providerError.Message)
		if errorCode != "" && errorMessage != "" {
			return volcengineQuotaUsageRefreshBody{}, fmt.Errorf("火山引擎返回错误: %s: %s", errorCode, errorMessage)
		}
		return volcengineQuotaUsageRefreshBody{}, fmt.Errorf("火山引擎返回错误: %s", firstNonEmpty(errorMessage, errorCode, "未知错误"))
	}
	if response.Result == nil {
		return volcengineQuotaUsageRefreshBody{}, fmt.Errorf("火山引擎额度响应缺少 Result")
	}
	return volcengineQuotaUsageRefreshBody{
		PlanType: strings.TrimSpace(response.Result.PlanType),
		FiveHour: normalizeVolcengineQuotaWindow(response.Result.FiveHourAFP),
		Daily:    normalizeVolcengineQuotaWindow(response.Result.DailyAFP),
		Weekly:   normalizeVolcengineQuotaWindow(response.Result.WeeklyAFP),
		Monthly:  normalizeVolcengineQuotaWindow(response.Result.MonthlyAFP),
	}, nil
}

func normalizeVolcengineQuotaWindow(window volcengineQuotaWindow) volcengineQuotaWindow {
	if window.Quota < 0 {
		window.Quota = 0
	}
	if window.UsedAFP < 0 || math.IsNaN(window.UsedAFP) || math.IsInf(window.UsedAFP, 0) {
		window.UsedAFP = 0
	}
	window.SubscribeTime = volcengineQuotaTimestampSeconds(window.SubscribeTime)
	window.ResetTime = volcengineQuotaTimestampSeconds(window.ResetTime)
	return window
}

func volcengineQuotaTimestampSeconds(timestamp int64) int64 {
	if timestamp <= 0 {
		return 0
	}
	if timestamp >= 1_000_000_000_000 {
		return timestamp / 1000
	}
	return timestamp
}

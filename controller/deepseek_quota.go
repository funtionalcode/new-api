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
	"github.com/shopspring/decimal"
)

type deepSeekQuotaBindingRequest struct {
	Name            string  `json:"name"`
	Note            string  `json:"note"`
	RequestCurl     string  `json:"request_curl"`
	UsageAmountCurl string  `json:"usage_amount_curl"`
	UsageCostCurl   string  `json:"usage_cost_curl"`
	Proxy           *string `json:"proxy"`
	Enabled         bool    `json:"enabled"`
}

type deepSeekQuotaResponse[T any] struct {
	Code int                          `json:"code"`
	Msg  string                       `json:"msg"`
	Data deepSeekQuotaResponseData[T] `json:"data"`
}

type deepSeekQuotaResponseData[T any] struct {
	BizCode int    `json:"biz_code"`
	BizMsg  string `json:"biz_msg"`
	BizData T      `json:"biz_data"`
}

type deepSeekQuotaSummaryBizData struct {
	CurrentToken                  int64                       `json:"current_token"`
	MonthlyUsage                  string                      `json:"monthly_usage"`
	TotalUsage                    int64                       `json:"total_usage"`
	NormalWallets                 []deepSeekQuotaWallet       `json:"normal_wallets"`
	BonusWallets                  []deepSeekQuotaWallet       `json:"bonus_wallets"`
	TotalAvailableTokenEstimation string                      `json:"total_available_token_estimation"`
	MonthlyCosts                  []deepSeekQuotaCurrencyCost `json:"monthly_costs"`
	TodayCosts                    []deepSeekQuotaCurrencyCost `json:"today_costs"`
	DailyCosts                    []deepSeekQuotaCurrencyCost `json:"daily_costs"`
	TotalCosts                    []deepSeekQuotaCurrencyCost `json:"total_costs"`
	MonthlyTokenUsage             string                      `json:"monthly_token_usage"`
	TodayTokenUsage               any                         `json:"today_token_usage"`
	DailyTokenUsage               any                         `json:"daily_token_usage"`
}

type deepSeekQuotaWallet struct {
	Currency        string `json:"currency"`
	Balance         string `json:"balance"`
	TokenEstimation string `json:"token_estimation"`
}

type deepSeekQuotaCurrencyCost struct {
	Currency string `json:"currency"`
	Amount   string `json:"amount"`
}

type deepSeekQuotaUsageAmountBizData struct {
	Start  int64                            `json:"start"`
	End    int64                            `json:"end"`
	Series []deepSeekQuotaUsageAmountSeries `json:"series"`
}

type deepSeekQuotaUsageAmountSeries struct {
	Buckets []deepSeekQuotaUsageAmountBucket `json:"buckets"`
}

type deepSeekQuotaUsageAmountBucket struct {
	Usage deepSeekQuotaUsageAmount `json:"usage"`
}

type deepSeekQuotaUsageAmount struct {
	ResponseToken        int64 `json:"RESPONSE_TOKEN"`
	Request              int64 `json:"REQUEST"`
	PromptCacheHitToken  int64 `json:"PROMPT_CACHE_HIT_TOKEN"`
	PromptCacheMissToken int64 `json:"PROMPT_CACHE_MISS_TOKEN"`
}

type deepSeekQuotaUsageCostBizData struct {
	Start int64                            `json:"start"`
	End   int64                            `json:"end"`
	Data  []deepSeekQuotaUsageCostCurrency `json:"data"`
}

type deepSeekQuotaUsageCostCurrency struct {
	Currency string                         `json:"currency"`
	Series   []deepSeekQuotaUsageCostSeries `json:"series"`
}

type deepSeekQuotaUsageCostSeries struct {
	Buckets []deepSeekQuotaUsageCostBucket `json:"buckets"`
}

type deepSeekQuotaUsageCostBucket struct {
	Cost string `json:"cost"`
}

type deepSeekQuotaUsageRefreshBody struct {
	MonthlyLimitTokens     int64
	MonthlyUsedTokens      int64
	MonthlyRemainingTokens int64
	MonthlyPercent         int
	TotalAvailableTokens   int64
	TodayUsedTokens        int64
	NormalWallets          []deepSeekQuotaWallet
	BonusWallets           []deepSeekQuotaWallet
	MonthlyCosts           []deepSeekQuotaCurrencyCost
	TodayCosts             []deepSeekQuotaCurrencyCost
	TotalCosts             []deepSeekQuotaCurrencyCost
	RequestCount           int64
}

const deepSeekQuotaUsageRangeDays = 30

func GetDeepSeekQuotaBindings(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	query := model.DeepSeekQuotaBindingQuery{
		Keyword: c.Query("keyword"),
		Enabled: parseOptionalBool(c.Query("enabled")),
	}
	bindings, total, err := model.GetDeepSeekQuotaBindings(query, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	sanitizeDeepSeekQuotaBindingsForRole(bindings, c.GetInt("role"))
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(bindings)
	common.ApiSuccess(c, pageInfo)
}

func CreateDeepSeekQuotaBinding(c *gin.Context) {
	update, err := decodeDeepSeekQuotaBindingRequest(c, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	binding := &model.DeepSeekQuotaBinding{
		Name:            update.Name,
		Note:            update.Note,
		RequestCurl:     update.RequestCurl,
		UsageAmountCurl: update.UsageAmountCurl,
		UsageCostCurl:   update.UsageCostCurl,
		Proxy:           update.Proxy,
		Enabled:         update.Enabled,
	}
	if err := model.CreateDeepSeekQuotaBinding(binding); err != nil {
		common.ApiError(c, err)
		return
	}
	binding.HasCurl = strings.TrimSpace(binding.RequestCurl) != ""
	binding.HasUsageAmountCurl = strings.TrimSpace(binding.UsageAmountCurl) != ""
	binding.HasUsageCostCurl = strings.TrimSpace(binding.UsageCostCurl) != ""
	common.ApiSuccess(c, binding)
}

func UpdateDeepSeekQuotaBinding(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("DeepSeek 额度配置 ID 无效"))
		return
	}
	update, err := decodeDeepSeekQuotaBindingRequest(c, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	binding, err := model.UpdateDeepSeekQuotaBinding(id, update)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, binding)
}

func DeleteDeepSeekQuotaBinding(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("DeepSeek 额度配置 ID 无效"))
		return
	}
	if err := model.DeleteDeepSeekQuotaBindingById(id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func RefreshDeepSeekQuotaBindingUsage(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("DeepSeek 额度配置 ID 无效"))
		return
	}
	binding, err := model.GetDeepSeekQuotaBindingById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !binding.Enabled {
		common.ApiError(c, fmt.Errorf("DeepSeek 额度配置已禁用"))
		return
	}
	usage, err := refreshDeepSeekQuotaUsage(c.Request.Context(), binding)
	if err != nil {
		updatedBinding, updateErr := model.UpdateDeepSeekQuotaBindingUsage(id, model.DeepSeekQuotaUsageRefreshUpdate{LastError: err.Error()})
		if updateErr != nil {
			common.ApiError(c, fmt.Errorf("刷新额度失败: %s；保存错误失败: %w", err.Error(), updateErr))
			return
		}
		sanitizeDeepSeekQuotaBindingForRole(updatedBinding, c.GetInt("role"))
		common.ApiSuccess(c, updatedBinding)
		return
	}
	normalWallets, err := common.Marshal(usage.NormalWallets)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	bonusWallets, err := common.Marshal(usage.BonusWallets)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	monthlyCosts, err := common.Marshal(usage.MonthlyCosts)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	todayCosts, err := common.Marshal(usage.TodayCosts)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	totalCosts, err := common.Marshal(usage.TotalCosts)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	updatedBinding, err := model.UpdateDeepSeekQuotaBindingUsage(id, model.DeepSeekQuotaUsageRefreshUpdate{
		LastMonthlyLimitTokens:     usage.MonthlyLimitTokens,
		LastMonthlyUsedTokens:      usage.MonthlyUsedTokens,
		LastMonthlyRemainingTokens: usage.MonthlyRemainingTokens,
		LastMonthlyPercent:         usage.MonthlyPercent,
		LastTotalAvailableTokens:   usage.TotalAvailableTokens,
		LastTodayUsedTokens:        usage.TodayUsedTokens,
		LastNormalWallets:          string(normalWallets),
		LastBonusWallets:           string(bonusWallets),
		LastMonthlyCosts:           string(monthlyCosts),
		LastTodayCosts:             string(todayCosts),
		LastTotalCosts:             string(totalCosts),
		LastRequestCount:           usage.RequestCount,
		LastError:                  "",
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	sanitizeDeepSeekQuotaBindingForRole(updatedBinding, c.GetInt("role"))
	common.ApiSuccess(c, updatedBinding)
}

func sanitizeDeepSeekQuotaBindingsForRole(bindings []*model.DeepSeekQuotaBinding, role int) {
	for _, binding := range bindings {
		sanitizeDeepSeekQuotaBindingForRole(binding, role)
	}
}

func sanitizeDeepSeekQuotaBindingForRole(binding *model.DeepSeekQuotaBinding, role int) {
	if binding == nil || role >= common.RoleAdminUser {
		return
	}
	binding.RequestCurl = ""
	binding.UsageAmountCurl = ""
	binding.UsageCostCurl = ""
	binding.Proxy = ""
}

func decodeDeepSeekQuotaBindingRequest(c *gin.Context, requireCurl bool) (model.DeepSeekQuotaBindingUpdate, error) {
	var request deepSeekQuotaBindingRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		return model.DeepSeekQuotaBindingUpdate{}, fmt.Errorf("无效的参数")
	}
	update := model.DeepSeekQuotaBindingUpdate{
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
	if err := model.ValidateDeepSeekQuotaBindingUpdate(update, requireCurl); err != nil {
		return model.DeepSeekQuotaBindingUpdate{}, err
	}
	return update, nil
}

func refreshDeepSeekQuotaUsage(ctx context.Context, binding *model.DeepSeekQuotaBinding) (deepSeekQuotaUsageRefreshBody, error) {
	summaryRequest, err := buildDeepSeekQuotaCurlRequest(binding.RequestCurl)
	if err != nil {
		return deepSeekQuotaUsageRefreshBody{}, fmt.Errorf("解析 DeepSeek 账户汇总 curl 失败: %w", err)
	}
	summaryBody, err := requestDeepSeekQuota(ctx, binding, summaryRequest)
	if err != nil {
		return deepSeekQuotaUsageRefreshBody{}, fmt.Errorf("获取 DeepSeek 账户汇总失败: %w", err)
	}

	usageAmountCurl := strings.TrimSpace(binding.UsageAmountCurl)
	usageCostCurl := strings.TrimSpace(binding.UsageCostCurl)
	if usageAmountCurl == "" && usageCostCurl == "" {
		return extractDeepSeekQuotaUsage(summaryBody, nil, nil)
	}
	if usageAmountCurl == "" {
		return deepSeekQuotaUsageRefreshBody{}, fmt.Errorf("DeepSeek 用量统计 curl 未配置")
	}
	if usageCostCurl == "" {
		return deepSeekQuotaUsageRefreshBody{}, fmt.Errorf("DeepSeek 消费统计 curl 未配置")
	}

	now := time.Now()
	amountRequest, err := buildDeepSeekQuotaUsageCurlRequest(usageAmountCurl, now)
	if err != nil {
		return deepSeekQuotaUsageRefreshBody{}, fmt.Errorf("解析 DeepSeek 用量统计 curl 失败: %w", err)
	}
	costRequest, err := buildDeepSeekQuotaUsageCurlRequest(usageCostCurl, now)
	if err != nil {
		return deepSeekQuotaUsageRefreshBody{}, fmt.Errorf("解析 DeepSeek 消费统计 curl 失败: %w", err)
	}
	amountBody, err := requestDeepSeekQuota(ctx, binding, amountRequest)
	if err != nil {
		return deepSeekQuotaUsageRefreshBody{}, fmt.Errorf("获取 DeepSeek 用量统计失败: %w", err)
	}
	costBody, err := requestDeepSeekQuota(ctx, binding, costRequest)
	if err != nil {
		return deepSeekQuotaUsageRefreshBody{}, fmt.Errorf("获取 DeepSeek 消费统计失败: %w", err)
	}
	return extractDeepSeekQuotaUsage(summaryBody, amountBody, costBody)
}

func requestDeepSeekQuota(ctx context.Context, binding *model.DeepSeekQuotaBinding, requestConfig quotaCurlRequest) ([]byte, error) {
	var requestBody io.Reader
	if requestConfig.Body != "" {
		requestBody = strings.NewReader(requestConfig.Body)
	}
	request, err := http.NewRequestWithContext(ctx, requestConfig.Method, requestConfig.URL, requestBody)
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
	proxyURL := resolveQuotaProxy(binding.Proxy, requestConfig.Proxy)
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
		return nil, fmt.Errorf("DeepSeek 返回 HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, nil
}

func buildDeepSeekQuotaCurlRequest(rawCurl string) (quotaCurlRequest, error) {
	return parseQuotaCurlRequest(rawCurl)
}

func buildDeepSeekQuotaUsageCurlRequest(rawCurl string, now time.Time) (quotaCurlRequest, error) {
	requestConfig, err := buildDeepSeekQuotaCurlRequest(rawCurl)
	if err != nil {
		return quotaCurlRequest{}, err
	}
	parsedURL, err := url.Parse(requestConfig.URL)
	if err != nil {
		return quotaCurlRequest{}, err
	}
	end := now.UTC().Truncate(24 * time.Hour).Add(24 * time.Hour)
	start := end.Add(-deepSeekQuotaUsageRangeDays * 24 * time.Hour)
	query := parsedURL.Query()
	query.Set("start", strconv.FormatInt(start.Unix(), 10))
	query.Set("end", strconv.FormatInt(end.Unix(), 10))
	query.Set("tz", "0")
	parsedURL.RawQuery = query.Encode()
	requestConfig.URL = parsedURL.String()
	return requestConfig, nil
}

func extractDeepSeekQuotaUsage(summaryBody []byte, amountBody []byte, costBody []byte) (deepSeekQuotaUsageRefreshBody, error) {
	summary, err := decodeDeepSeekQuotaResponse[deepSeekQuotaSummaryBizData](summaryBody)
	if err != nil {
		return deepSeekQuotaUsageRefreshBody{}, err
	}
	monthlyUsedTokens := int64FromNumericString(firstNonEmpty(summary.MonthlyTokenUsage, summary.MonthlyUsage))
	remainingTokens := int64(0)
	if summary.CurrentToken > monthlyUsedTokens {
		remainingTokens = summary.CurrentToken - monthlyUsedTokens
	}
	usage := deepSeekQuotaUsageRefreshBody{
		MonthlyLimitTokens:     summary.CurrentToken,
		MonthlyUsedTokens:      monthlyUsedTokens,
		MonthlyRemainingTokens: remainingTokens,
		MonthlyPercent:         deepSeekQuotaUsagePercent(monthlyUsedTokens, summary.CurrentToken),
		TotalAvailableTokens:   int64FromNumericString(summary.TotalAvailableTokenEstimation),
		TodayUsedTokens:        firstPositiveInt64FromNumericValues(summary.TodayTokenUsage, summary.DailyTokenUsage),
		NormalWallets:          summary.NormalWallets,
		BonusWallets:           summary.BonusWallets,
		MonthlyCosts:           summary.MonthlyCosts,
		TodayCosts:             firstDeepSeekQuotaCostList(summary.TodayCosts, summary.DailyCosts),
		TotalCosts:             summary.TotalCosts,
	}
	if len(amountBody) == 0 && len(costBody) == 0 {
		return usage, nil
	}
	if len(amountBody) == 0 || len(costBody) == 0 {
		return deepSeekQuotaUsageRefreshBody{}, fmt.Errorf("DeepSeek 用量与消费响应必须同时提供")
	}

	amountData, err := decodeDeepSeekQuotaResponse[deepSeekQuotaUsageAmountBizData](amountBody)
	if err != nil {
		return deepSeekQuotaUsageRefreshBody{}, err
	}
	usage.MonthlyUsedTokens = 0
	for _, series := range amountData.Series {
		for _, bucket := range series.Buckets {
			usage.RequestCount = addDeepSeekUsageCount(usage.RequestCount, bucket.Usage.Request)
			usage.MonthlyUsedTokens = addDeepSeekUsageCount(usage.MonthlyUsedTokens, bucket.Usage.ResponseToken)
			usage.MonthlyUsedTokens = addDeepSeekUsageCount(usage.MonthlyUsedTokens, bucket.Usage.PromptCacheHitToken)
			usage.MonthlyUsedTokens = addDeepSeekUsageCount(usage.MonthlyUsedTokens, bucket.Usage.PromptCacheMissToken)
		}
	}
	usage.MonthlyRemainingTokens = 0
	if usage.MonthlyLimitTokens > usage.MonthlyUsedTokens {
		usage.MonthlyRemainingTokens = usage.MonthlyLimitTokens - usage.MonthlyUsedTokens
	}
	usage.MonthlyPercent = deepSeekQuotaUsagePercent(usage.MonthlyUsedTokens, usage.MonthlyLimitTokens)
	costData, err := decodeDeepSeekQuotaResponse[deepSeekQuotaUsageCostBizData](costBody)
	if err != nil {
		return deepSeekQuotaUsageRefreshBody{}, err
	}
	usage.MonthlyCosts, err = aggregateDeepSeekQuotaCosts(costData.Data)
	if err != nil {
		return deepSeekQuotaUsageRefreshBody{}, err
	}
	return usage, nil
}

func decodeDeepSeekQuotaResponse[T any](body []byte) (T, error) {
	var zero T
	var response deepSeekQuotaResponse[T]
	if err := common.Unmarshal(body, &response); err != nil {
		return zero, err
	}
	if response.Code != 0 {
		return zero, fmt.Errorf("DeepSeek 返回错误: %s", firstNonEmpty(response.Msg, strconv.Itoa(response.Code)))
	}
	if response.Data.BizCode != 0 {
		return zero, fmt.Errorf("DeepSeek 返回业务错误: %s", firstNonEmpty(response.Data.BizMsg, strconv.Itoa(response.Data.BizCode)))
	}
	return response.Data.BizData, nil
}

func aggregateDeepSeekQuotaCosts(values []deepSeekQuotaUsageCostCurrency) ([]deepSeekQuotaCurrencyCost, error) {
	amounts := make(map[string]decimal.Decimal, len(values))
	currencies := make([]string, 0, len(values))
	for _, value := range values {
		currency := strings.TrimSpace(value.Currency)
		if _, exists := amounts[currency]; !exists {
			amounts[currency] = decimal.Zero
			currencies = append(currencies, currency)
		}
		for _, series := range value.Series {
			for _, bucket := range series.Buckets {
				cost := strings.TrimSpace(bucket.Cost)
				if cost == "" {
					continue
				}
				parsedCost, err := decimal.NewFromString(cost)
				if err != nil {
					return nil, fmt.Errorf("DeepSeek 消费金额无效: %w", err)
				}
				amounts[currency] = amounts[currency].Add(parsedCost)
			}
		}
	}
	result := make([]deepSeekQuotaCurrencyCost, 0, len(currencies))
	for _, currency := range currencies {
		result = append(result, deepSeekQuotaCurrencyCost{
			Currency: currency,
			Amount:   amounts[currency].String(),
		})
	}
	return result, nil
}

func addDeepSeekUsageCount(total int64, value int64) int64 {
	if value <= 0 {
		return total
	}
	if total > (1<<63-1)-value {
		return 1<<63 - 1
	}
	return total + value
}

func firstDeepSeekQuotaCostList(values ...[]deepSeekQuotaCurrencyCost) []deepSeekQuotaCurrencyCost {
	for _, value := range values {
		if len(value) > 0 {
			return value
		}
	}
	return nil
}

func int64FromNumericString(value string) int64 {
	trimmedValue := strings.TrimSpace(value)
	parsedValue, err := strconv.ParseInt(trimmedValue, 10, 64)
	if err == nil {
		return parsedValue
	}
	parsedFloatValue, _ := strconv.ParseFloat(trimmedValue, 64)
	return int64(math.Round(parsedFloatValue))
}

func int64FromNumericValue(value any) int64 {
	switch typedValue := value.(type) {
	case nil:
		return 0
	case string:
		return int64FromNumericString(typedValue)
	case float64:
		return int64(math.Round(typedValue))
	case float32:
		return int64(math.Round(float64(typedValue)))
	case int:
		return int64(typedValue)
	case int64:
		return typedValue
	case int32:
		return int64(typedValue)
	case uint:
		return int64(typedValue)
	case uint64:
		if typedValue > uint64(1<<63-1) {
			return 1<<63 - 1
		}
		return int64(typedValue)
	case uint32:
		return int64(typedValue)
	default:
		return 0
	}
}

func firstPositiveInt64FromNumericValues(values ...any) int64 {
	for _, value := range values {
		parsedValue := int64FromNumericValue(value)
		if parsedValue > 0 {
			return parsedValue
		}
	}
	return 0
}

func deepSeekQuotaUsagePercent(usedTokens int64, limitTokens int64) int {
	if limitTokens <= 0 {
		return 0
	}
	return int(math.Round(float64(usedTokens) * 100 / float64(limitTokens)))
}

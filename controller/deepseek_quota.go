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
	"unicode"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

type deepSeekQuotaBindingRequest struct {
	Name        string  `json:"name"`
	Note        string  `json:"note"`
	RequestCurl string  `json:"request_curl"`
	Proxy       *string `json:"proxy"`
	Enabled     bool    `json:"enabled"`
}

type deepSeekQuotaCurlRequest struct {
	Method  string
	URL     string
	Headers map[string]string
	Proxy   string
}

type deepSeekQuotaSummaryResponse struct {
	Code int                      `json:"code"`
	Msg  string                   `json:"msg"`
	Data deepSeekQuotaSummaryData `json:"data"`
}

type deepSeekQuotaSummaryData struct {
	BizCode int                         `json:"biz_code"`
	BizMsg  string                      `json:"biz_msg"`
	BizData deepSeekQuotaSummaryBizData `json:"biz_data"`
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
}

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
		Name:        update.Name,
		Note:        update.Note,
		RequestCurl: update.RequestCurl,
		Proxy:       update.Proxy,
		Enabled:     update.Enabled,
	}
	if err := model.CreateDeepSeekQuotaBinding(binding); err != nil {
		common.ApiError(c, err)
		return
	}
	binding.HasCurl = strings.TrimSpace(binding.RequestCurl) != ""
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
	binding.Proxy = ""
}

func decodeDeepSeekQuotaBindingRequest(c *gin.Context, requireCurl bool) (model.DeepSeekQuotaBindingUpdate, error) {
	var request deepSeekQuotaBindingRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		return model.DeepSeekQuotaBindingUpdate{}, fmt.Errorf("无效的参数")
	}
	update := model.DeepSeekQuotaBindingUpdate{
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
	if err := model.ValidateDeepSeekQuotaBindingUpdate(update, requireCurl); err != nil {
		return model.DeepSeekQuotaBindingUpdate{}, err
	}
	return update, nil
}

func refreshDeepSeekQuotaUsage(ctx context.Context, binding *model.DeepSeekQuotaBinding) (deepSeekQuotaUsageRefreshBody, error) {
	requestConfig, err := buildDeepSeekQuotaCurlRequest(binding.RequestCurl)
	if err != nil {
		return deepSeekQuotaUsageRefreshBody{}, err
	}
	request, err := http.NewRequestWithContext(ctx, requestConfig.Method, requestConfig.URL, nil)
	if err != nil {
		return deepSeekQuotaUsageRefreshBody{}, err
	}
	for key, value := range requestConfig.Headers {
		if strings.EqualFold(key, "host") {
			request.Host = value
			continue
		}
		if shouldSkipDeepSeekQuotaHeader(key) {
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
			return deepSeekQuotaUsageRefreshBody{}, fmt.Errorf("创建代理客户端 %s 失败: %w", quotaProxyLabel(proxyURL), err)
		}
		return deepSeekQuotaUsageRefreshBody{}, err
	}
	response, err := client.Do(request)
	if err != nil {
		return deepSeekQuotaUsageRefreshBody{}, quotaHTTPRequestError(proxyURL, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return deepSeekQuotaUsageRefreshBody{}, err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return deepSeekQuotaUsageRefreshBody{}, fmt.Errorf("DeepSeek 返回 HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	return extractDeepSeekQuotaUsage(body)
}

func buildDeepSeekQuotaCurlRequest(rawCurl string) (deepSeekQuotaCurlRequest, error) {
	tokens, err := splitDeepSeekQuotaCurlCommand(rawCurl)
	if err != nil {
		return deepSeekQuotaCurlRequest{}, err
	}
	if len(tokens) == 0 || strings.ToLower(tokens[0]) != "curl" {
		return deepSeekQuotaCurlRequest{}, fmt.Errorf("请输入完整的 curl 命令")
	}
	requestConfig := deepSeekQuotaCurlRequest{
		Method:  http.MethodGet,
		Headers: map[string]string{},
	}
	for index := 1; index < len(tokens); index++ {
		token := tokens[index]
		switch token {
		case "-H", "--header":
			index++
			if index >= len(tokens) {
				return deepSeekQuotaCurlRequest{}, fmt.Errorf("curl header 参数缺少值")
			}
			addDeepSeekQuotaHeader(requestConfig.Headers, tokens[index])
		case "-b", "--cookie":
			index++
			if index >= len(tokens) {
				return deepSeekQuotaCurlRequest{}, fmt.Errorf("curl cookie 参数缺少值")
			}
			appendDeepSeekQuotaCookie(requestConfig.Headers, tokens[index])
		case "-X", "--request":
			index++
			if index >= len(tokens) {
				return deepSeekQuotaCurlRequest{}, fmt.Errorf("curl request 参数缺少值")
			}
			requestConfig.Method = strings.ToUpper(strings.TrimSpace(tokens[index]))
		case "--url":
			index++
			if index >= len(tokens) {
				return deepSeekQuotaCurlRequest{}, fmt.Errorf("curl url 参数缺少值")
			}
			requestConfig.URL = tokens[index]
		case "-x", "--proxy":
			index++
			if index >= len(tokens) {
				return deepSeekQuotaCurlRequest{}, fmt.Errorf("curl proxy 参数缺少值")
			}
			requestConfig.Proxy = strings.TrimSpace(tokens[index])
		default:
			if proxyURL, ok := quotaCurlInlineProxy(token); ok {
				requestConfig.Proxy = proxyURL
				continue
			}
			if requestConfig.URL == "" && strings.HasPrefix(token, "http") {
				requestConfig.URL = token
			}
		}
	}
	if requestConfig.URL == "" {
		return deepSeekQuotaCurlRequest{}, fmt.Errorf("curl 中未找到请求地址")
	}
	parsedURL, err := url.Parse(requestConfig.URL)
	if err != nil {
		return deepSeekQuotaCurlRequest{}, err
	}
	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return deepSeekQuotaCurlRequest{}, fmt.Errorf("curl 请求地址无效")
	}
	requestConfig.URL = parsedURL.String()
	return requestConfig, nil
}

func splitDeepSeekQuotaCurlCommand(rawCurl string) ([]string, error) {
	input := strings.ReplaceAll(rawCurl, "\\\r\n", " ")
	input = strings.ReplaceAll(input, "\\\n", " ")
	var tokens []string
	var builder strings.Builder
	var quote rune
	escaped := false
	inToken := false
	for _, char := range input {
		if escaped {
			builder.WriteRune(char)
			escaped = false
			inToken = true
			continue
		}
		if char == '\\' && quote != '\'' {
			escaped = true
			inToken = true
			continue
		}
		if quote != 0 {
			if char == quote {
				quote = 0
				continue
			}
			builder.WriteRune(char)
			inToken = true
			continue
		}
		if char == '\'' || char == '"' {
			quote = char
			inToken = true
			continue
		}
		if unicode.IsSpace(char) {
			if inToken {
				tokens = append(tokens, builder.String())
				builder.Reset()
				inToken = false
			}
			continue
		}
		builder.WriteRune(char)
		inToken = true
	}
	if escaped {
		builder.WriteRune('\\')
	}
	if quote != 0 {
		return nil, fmt.Errorf("curl 引号未闭合")
	}
	if inToken {
		tokens = append(tokens, builder.String())
	}
	return tokens, nil
}

func addDeepSeekQuotaHeader(headers map[string]string, rawHeader string) {
	parts := strings.SplitN(rawHeader, ":", 2)
	if len(parts) != 2 {
		return
	}
	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])
	if key == "" {
		return
	}
	if strings.EqualFold(key, "cookie") {
		appendDeepSeekQuotaCookie(headers, value)
		return
	}
	headers[key] = value
}

func appendDeepSeekQuotaCookie(headers map[string]string, cookie string) {
	cookie = strings.TrimSpace(cookie)
	if cookie == "" {
		return
	}
	for key, value := range headers {
		if strings.EqualFold(key, "cookie") {
			if strings.TrimSpace(value) == "" {
				headers[key] = cookie
			} else {
				headers[key] = value + "; " + cookie
			}
			return
		}
	}
	headers["Cookie"] = cookie
}

func shouldSkipDeepSeekQuotaHeader(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "content-length", "connection":
		return true
	default:
		return false
	}
}

func extractDeepSeekQuotaUsage(body []byte) (deepSeekQuotaUsageRefreshBody, error) {
	var response deepSeekQuotaSummaryResponse
	if err := common.Unmarshal(body, &response); err != nil {
		return deepSeekQuotaUsageRefreshBody{}, err
	}
	if response.Code != 0 {
		return deepSeekQuotaUsageRefreshBody{}, fmt.Errorf("DeepSeek 返回错误: %s", firstNonEmpty(response.Msg, strconv.Itoa(response.Code)))
	}
	if response.Data.BizCode != 0 {
		return deepSeekQuotaUsageRefreshBody{}, fmt.Errorf("DeepSeek 返回业务错误: %s", firstNonEmpty(response.Data.BizMsg, strconv.Itoa(response.Data.BizCode)))
	}
	data := response.Data.BizData
	monthlyUsedTokens := int64FromNumericString(firstNonEmpty(data.MonthlyTokenUsage, data.MonthlyUsage))
	remainingTokens := data.CurrentToken - monthlyUsedTokens
	if remainingTokens < 0 {
		remainingTokens = 0
	}
	return deepSeekQuotaUsageRefreshBody{
		MonthlyLimitTokens:     data.CurrentToken,
		MonthlyUsedTokens:      monthlyUsedTokens,
		MonthlyRemainingTokens: remainingTokens,
		MonthlyPercent:         deepSeekQuotaUsagePercent(monthlyUsedTokens, data.CurrentToken),
		TotalAvailableTokens:   int64FromNumericString(data.TotalAvailableTokenEstimation),
		TodayUsedTokens:        firstPositiveInt64FromNumericValues(data.TodayTokenUsage, data.DailyTokenUsage),
		NormalWallets:          data.NormalWallets,
		BonusWallets:           data.BonusWallets,
		MonthlyCosts:           data.MonthlyCosts,
		TodayCosts:             firstDeepSeekQuotaCostList(data.TodayCosts, data.DailyCosts),
	}, nil
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

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
	"unicode"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

const (
	glmQuotaUsageTimeLayout       = "2006-01-02 15:04:05"
	glmQuotaUsageTimeMinuteLayout = "2006-01-02 15:04"
)

type glmQuotaBindingRequest struct {
	Name                string  `json:"name"`
	Note                string  `json:"note"`
	RequestCurl         string  `json:"request_curl"`
	Proxy               *string `json:"proxy"`
	PlanType            string  `json:"plan_type"`
	FiveHourLimitTokens int64   `json:"five_hour_limit_tokens"`
	WeeklyLimitTokens   int64   `json:"weekly_limit_tokens"`
	Enabled             bool    `json:"enabled"`
}

type glmQuotaUsageRequest struct {
	Method  string
	URL     string
	Headers map[string]string
}

type glmQuotaUsageRefreshBody struct {
	FiveHourUsedTokens int64
	WeeklyUsedTokens   int64
	ModelCallCount     int64
	ModelSummary       []glmQuotaModelSummary
}

type glmQuotaModelUsageResponse struct {
	Code    int               `json:"code"`
	Msg     string            `json:"msg"`
	Data    glmQuotaUsageData `json:"data"`
	Success bool              `json:"success"`
}

type glmQuotaUsageData struct {
	XTime            []string                    `json:"x_time"`
	TokensUsage      []int64                     `json:"tokensUsage"`
	TotalUsage       glmQuotaTotalUsage          `json:"totalUsage"`
	ModelSummaryList []glmQuotaModelSummary      `json:"modelSummaryList"`
	ModelDataList    []glmQuotaModelDataListItem `json:"modelDataList"`
}

type glmQuotaTotalUsage struct {
	TotalModelCallCount int64                  `json:"totalModelCallCount"`
	TotalTokensUsage    int64                  `json:"totalTokensUsage"`
	ModelSummaryList    []glmQuotaModelSummary `json:"modelSummaryList"`
}

type glmQuotaModelSummary struct {
	ModelName   string `json:"modelName"`
	TotalTokens int64  `json:"totalTokens"`
	SortOrder   int    `json:"sortOrder"`
}

type glmQuotaModelDataListItem struct {
	ModelName   string  `json:"modelName"`
	TotalTokens int64   `json:"totalTokens"`
	SortOrder   int     `json:"sortOrder"`
	TokensUsage []int64 `json:"tokensUsage"`
}

type glmQuotaPlanSpec struct {
	Label               string
	FiveHourLimitTokens int64
	WeeklyLimitTokens   int64
}

var glmQuotaPlanSpecs = map[string]glmQuotaPlanSpec{
	"standard": {
		Label:               "标准版",
		FiveHourLimitTokens: 60_000_000,
		WeeklyLimitTokens:   300_000_000,
	},
	"标准版": {
		Label:               "标准版",
		FiveHourLimitTokens: 60_000_000,
		WeeklyLimitTokens:   300_000_000,
	},
	"advanced": {
		Label:               "高级版",
		FiveHourLimitTokens: 160_000_000,
		WeeklyLimitTokens:   800_000_000,
	},
	"高级版": {
		Label:               "高级版",
		FiveHourLimitTokens: 160_000_000,
		WeeklyLimitTokens:   800_000_000,
	},
}

func GetGLMQuotaBindings(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	query := model.GLMQuotaBindingQuery{
		Keyword: c.Query("keyword"),
		Enabled: parseOptionalBool(c.Query("enabled")),
	}
	bindings, total, err := model.GetGLMQuotaBindings(query, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	sanitizeGLMQuotaBindingsForRole(bindings, c.GetInt("role"))
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(bindings)
	common.ApiSuccess(c, pageInfo)
}

func CreateGLMQuotaBinding(c *gin.Context) {
	update, err := decodeGLMQuotaBindingRequest(c, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	binding := &model.GLMQuotaBinding{
		Name:                update.Name,
		Note:                update.Note,
		RequestCurl:         update.RequestCurl,
		Proxy:               update.Proxy,
		PlanType:            update.PlanType,
		FiveHourLimitTokens: update.FiveHourLimitTokens,
		WeeklyLimitTokens:   update.WeeklyLimitTokens,
		Enabled:             update.Enabled,
	}
	if err := model.CreateGLMQuotaBinding(binding); err != nil {
		common.ApiError(c, err)
		return
	}
	binding.HasCurl = strings.TrimSpace(binding.RequestCurl) != ""
	common.ApiSuccess(c, binding)
}

func UpdateGLMQuotaBinding(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("GLM 额度配置 ID 无效"))
		return
	}
	update, err := decodeGLMQuotaBindingRequest(c, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	binding, err := model.UpdateGLMQuotaBinding(id, update)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, binding)
}

func DeleteGLMQuotaBinding(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("GLM 额度配置 ID 无效"))
		return
	}
	if err := model.DeleteGLMQuotaBindingById(id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func RefreshGLMQuotaBindingUsage(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("GLM 额度配置 ID 无效"))
		return
	}
	binding, err := model.GetGLMQuotaBindingById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !binding.Enabled {
		common.ApiError(c, fmt.Errorf("GLM 额度配置已禁用"))
		return
	}
	usage, err := refreshGLMQuotaUsage(c.Request.Context(), binding, time.Now())
	if err != nil {
		updatedBinding, updateErr := model.UpdateGLMQuotaBindingUsage(id, model.GLMQuotaUsageRefreshUpdate{LastError: err.Error()})
		if updateErr != nil {
			common.ApiError(c, fmt.Errorf("刷新额度失败: %s；保存错误失败: %w", err.Error(), updateErr))
			return
		}
		sanitizeGLMQuotaBindingForRole(updatedBinding, c.GetInt("role"))
		common.ApiSuccess(c, updatedBinding)
		return
	}
	modelSummary, err := common.Marshal(usage.ModelSummary)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	updatedBinding, err := model.UpdateGLMQuotaBindingUsage(id, model.GLMQuotaUsageRefreshUpdate{
		LastFiveHourUsedTokens: usage.FiveHourUsedTokens,
		LastWeeklyUsedTokens:   usage.WeeklyUsedTokens,
		LastFiveHourPercent:    glmQuotaUsagePercent(usage.FiveHourUsedTokens, binding.FiveHourLimitTokens),
		LastWeeklyPercent:      glmQuotaUsagePercent(usage.WeeklyUsedTokens, binding.WeeklyLimitTokens),
		LastModelCallCount:     usage.ModelCallCount,
		LastModelSummary:       string(modelSummary),
		LastError:              "",
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	sanitizeGLMQuotaBindingForRole(updatedBinding, c.GetInt("role"))
	common.ApiSuccess(c, updatedBinding)
}

func sanitizeGLMQuotaBindingsForRole(bindings []*model.GLMQuotaBinding, role int) {
	for _, binding := range bindings {
		sanitizeGLMQuotaBindingForRole(binding, role)
	}
}

func sanitizeGLMQuotaBindingForRole(binding *model.GLMQuotaBinding, role int) {
	if binding == nil || role >= common.RoleAdminUser {
		return
	}
	binding.RequestCurl = ""
	binding.Proxy = ""
}

func decodeGLMQuotaBindingRequest(c *gin.Context, requireCurl bool) (model.GLMQuotaBindingUpdate, error) {
	var request glmQuotaBindingRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		return model.GLMQuotaBindingUpdate{}, fmt.Errorf("无效的参数")
	}
	update := model.GLMQuotaBindingUpdate{
		Name:                strings.TrimSpace(request.Name),
		Note:                strings.TrimSpace(request.Note),
		RequestCurl:         strings.TrimSpace(request.RequestCurl),
		UpdateCurl:          strings.TrimSpace(request.RequestCurl) != "",
		UpdateProxy:         request.Proxy != nil,
		PlanType:            strings.TrimSpace(request.PlanType),
		FiveHourLimitTokens: request.FiveHourLimitTokens,
		WeeklyLimitTokens:   request.WeeklyLimitTokens,
		Enabled:             request.Enabled,
	}
	if request.Proxy != nil {
		update.Proxy = strings.TrimSpace(*request.Proxy)
	}
	applyGLMQuotaPlanSpecDefaults(&update)
	if err := model.ValidateGLMQuotaBindingUpdate(update, requireCurl); err != nil {
		return model.GLMQuotaBindingUpdate{}, err
	}
	return update, nil
}

func applyGLMQuotaPlanSpecDefaults(update *model.GLMQuotaBindingUpdate) {
	if update == nil {
		return
	}
	spec, ok := glmQuotaPlanSpecs[strings.TrimSpace(update.PlanType)]
	if !ok {
		return
	}
	update.PlanType = spec.Label
	if update.FiveHourLimitTokens <= 0 {
		update.FiveHourLimitTokens = spec.FiveHourLimitTokens
	}
	if update.WeeklyLimitTokens <= 0 {
		update.WeeklyLimitTokens = spec.WeeklyLimitTokens
	}
}

func refreshGLMQuotaUsage(ctx context.Context, binding *model.GLMQuotaBinding, now time.Time) (glmQuotaUsageRefreshBody, error) {
	requestConfig, err := buildGLMQuotaUsageRequest(binding.RequestCurl, now)
	if err != nil {
		return glmQuotaUsageRefreshBody{}, err
	}
	request, err := http.NewRequestWithContext(ctx, requestConfig.Method, requestConfig.URL, nil)
	if err != nil {
		return glmQuotaUsageRefreshBody{}, err
	}
	for key, value := range requestConfig.Headers {
		if strings.EqualFold(key, "host") {
			request.Host = value
			continue
		}
		if shouldSkipGLMQuotaHeader(key) {
			continue
		}
		request.Header.Set(key, value)
	}
	if request.Header.Get("User-Agent") == "" {
		request.Header.Set("User-Agent", "Mozilla/5.0")
	}
	client, err := quotaHTTPClient(binding.Proxy)
	if err != nil {
		return glmQuotaUsageRefreshBody{}, err
	}
	response, err := client.Do(request)
	if err != nil {
		return glmQuotaUsageRefreshBody{}, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return glmQuotaUsageRefreshBody{}, err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return glmQuotaUsageRefreshBody{}, fmt.Errorf("BigModel 返回 HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	return extractGLMQuotaUsage(body, now)
}

func buildGLMQuotaUsageRequest(rawCurl string, now time.Time) (glmQuotaUsageRequest, error) {
	tokens, err := splitGLMQuotaCurlCommand(rawCurl)
	if err != nil {
		return glmQuotaUsageRequest{}, err
	}
	if len(tokens) == 0 || strings.ToLower(tokens[0]) != "curl" {
		return glmQuotaUsageRequest{}, fmt.Errorf("请输入完整的 curl 命令")
	}
	requestConfig := glmQuotaUsageRequest{
		Method:  http.MethodGet,
		Headers: map[string]string{},
	}
	for index := 1; index < len(tokens); index++ {
		token := tokens[index]
		switch token {
		case "-H", "--header":
			index++
			if index >= len(tokens) {
				return glmQuotaUsageRequest{}, fmt.Errorf("curl header 参数缺少值")
			}
			addGLMQuotaHeader(requestConfig.Headers, tokens[index])
		case "-b", "--cookie":
			index++
			if index >= len(tokens) {
				return glmQuotaUsageRequest{}, fmt.Errorf("curl cookie 参数缺少值")
			}
			appendGLMQuotaCookie(requestConfig.Headers, tokens[index])
		case "-X", "--request":
			index++
			if index >= len(tokens) {
				return glmQuotaUsageRequest{}, fmt.Errorf("curl request 参数缺少值")
			}
			requestConfig.Method = strings.ToUpper(strings.TrimSpace(tokens[index]))
		case "--url":
			index++
			if index >= len(tokens) {
				return glmQuotaUsageRequest{}, fmt.Errorf("curl url 参数缺少值")
			}
			requestConfig.URL = tokens[index]
		default:
			if requestConfig.URL == "" && strings.HasPrefix(token, "http") {
				requestConfig.URL = token
			}
		}
	}
	if requestConfig.URL == "" {
		return glmQuotaUsageRequest{}, fmt.Errorf("curl 中未找到请求地址")
	}
	normalizedURL, err := normalizeGLMQuotaUsageURL(requestConfig.URL, now)
	if err != nil {
		return glmQuotaUsageRequest{}, err
	}
	requestConfig.URL = normalizedURL
	return requestConfig, nil
}

func splitGLMQuotaCurlCommand(rawCurl string) ([]string, error) {
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

func addGLMQuotaHeader(headers map[string]string, rawHeader string) {
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
		appendGLMQuotaCookie(headers, value)
		return
	}
	headers[key] = value
}

func appendGLMQuotaCookie(headers map[string]string, cookie string) {
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

func shouldSkipGLMQuotaHeader(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "content-length", "connection":
		return true
	default:
		return false
	}
}

func normalizeGLMQuotaUsageURL(rawURL string, now time.Time) (string, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return "", fmt.Errorf("curl 请求地址无效")
	}
	startTime, endTime := glmQuotaUsageWindow(now)
	query := parsedURL.Query()
	query.Set("startTime", startTime.Format(glmQuotaUsageTimeLayout))
	query.Set("endTime", endTime.Format(glmQuotaUsageTimeLayout))
	parsedURL.RawQuery = query.Encode()
	return parsedURL.String(), nil
}

func glmQuotaUsageWindow(now time.Time) (time.Time, time.Time) {
	localNow := now.In(now.Location())
	start := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, localNow.Location()).AddDate(0, 0, -6)
	end := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 23, 59, 59, 0, localNow.Location())
	return start, end
}

func extractGLMQuotaUsage(body []byte, now time.Time) (glmQuotaUsageRefreshBody, error) {
	var response glmQuotaModelUsageResponse
	if err := common.Unmarshal(body, &response); err != nil {
		return glmQuotaUsageRefreshBody{}, err
	}
	if !response.Success && response.Code != http.StatusOK {
		if strings.TrimSpace(response.Msg) != "" {
			return glmQuotaUsageRefreshBody{}, fmt.Errorf("BigModel 返回错误: %s", response.Msg)
		}
		return glmQuotaUsageRefreshBody{}, fmt.Errorf("BigModel 返回错误")
	}
	data := response.Data
	modelSummary := data.TotalUsage.ModelSummaryList
	if len(modelSummary) == 0 {
		modelSummary = data.ModelSummaryList
	}
	if len(modelSummary) == 0 && len(data.ModelDataList) > 0 {
		for _, item := range data.ModelDataList {
			modelSummary = append(modelSummary, glmQuotaModelSummary{
				ModelName:   item.ModelName,
				TotalTokens: item.TotalTokens,
				SortOrder:   item.SortOrder,
			})
		}
	}
	weeklyUsedTokens := data.TotalUsage.TotalTokensUsage
	if weeklyUsedTokens == 0 {
		weeklyUsedTokens = sumGLMQuotaTokens(data.TokensUsage)
	}
	fiveHourUsedTokens := sumGLMQuotaFiveHourTokens(data.XTime, data.TokensUsage, now)
	if len(data.XTime) == 0 && fiveHourUsedTokens == 0 {
		fiveHourUsedTokens = sumLastGLMQuotaTokens(data.TokensUsage, 5)
	}
	return glmQuotaUsageRefreshBody{
		FiveHourUsedTokens: fiveHourUsedTokens,
		WeeklyUsedTokens:   weeklyUsedTokens,
		ModelCallCount:     data.TotalUsage.TotalModelCallCount,
		ModelSummary:       modelSummary,
	}, nil
}

func sumGLMQuotaFiveHourTokens(timeBuckets []string, tokenBuckets []int64, now time.Time) int64 {
	if len(timeBuckets) == 0 || len(tokenBuckets) == 0 {
		return 0
	}
	localNow := now.In(now.Location())
	windowEnd := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), localNow.Hour(), 0, 0, 0, localNow.Location())
	windowStart := windowEnd.Add(-4 * time.Hour)
	var total int64
	var matched int
	for index, rawTime := range timeBuckets {
		if index >= len(tokenBuckets) {
			break
		}
		parsedTime, ok := parseGLMQuotaBucketTime(rawTime, localNow.Location())
		if !ok {
			continue
		}
		if parsedTime.Before(windowStart) || parsedTime.After(windowEnd) {
			continue
		}
		total += tokenBuckets[index]
		matched++
	}
	if matched == 0 {
		return sumLastGLMQuotaTokens(tokenBuckets, 5)
	}
	return total
}

func parseGLMQuotaBucketTime(rawTime string, location *time.Location) (time.Time, bool) {
	rawTime = strings.TrimSpace(rawTime)
	if rawTime == "" {
		return time.Time{}, false
	}
	if parsedTime, err := time.ParseInLocation(glmQuotaUsageTimeLayout, rawTime, location); err == nil {
		return parsedTime, true
	}
	if parsedTime, err := time.ParseInLocation(glmQuotaUsageTimeMinuteLayout, rawTime, location); err == nil {
		return parsedTime, true
	}
	return time.Time{}, false
}

func sumGLMQuotaTokens(values []int64) int64 {
	var total int64
	for _, value := range values {
		total += value
	}
	return total
}

func sumLastGLMQuotaTokens(values []int64, count int) int64 {
	if count <= 0 || len(values) == 0 {
		return 0
	}
	start := len(values) - count
	if start < 0 {
		start = 0
	}
	return sumGLMQuotaTokens(values[start:])
}

func glmQuotaUsagePercent(usedTokens int64, limitTokens int64) int {
	if limitTokens <= 0 {
		return 0
	}
	return int(math.Round(float64(usedTokens) * 100 / float64(limitTokens)))
}

package controller

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type typeSafeUsageBucket struct {
	Day          string  `json:"day"`
	APIKeyID     *string `json:"apiKeyId"`
	APIKeyName   *string `json:"apiKeyName"`
	UserID       *string `json:"userId"`
	Requests     int64   `json:"requests"`
	InputTokens  int64   `json:"inputTokens"`
	OutputTokens int64   `json:"outputTokens"`
}

func GetTypeSafeUsageBindings(c *gin.Context) {
	page := common.GetPageQuery(c)
	bindings, total, err := model.GetTypeSafeUsageBindings(strings.TrimSpace(c.Query("keyword")), page.GetStartIdx(), page.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	page.SetTotal(int(total))
	page.SetItems(bindings)
	common.ApiSuccess(c, page)
}

func SaveTypeSafeUsageBinding(c *gin.Context) {
	var input struct {
		Name        string  `json:"name"`
		Note        string  `json:"note"`
		RequestCurl string  `json:"request_curl"`
		Proxy       *string `json:"proxy"`
		Enabled     bool    `json:"enabled"`
	}
	if err := common.DecodeJson(io.LimitReader(c.Request.Body, 128*1024), &input); err != nil {
		common.ApiError(c, errors.New("无效的 TypeSafe 用量配置"))
		return
	}
	binding := &model.TypeSafeUsageBinding{}
	if c.Param("id") != "" {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil || id <= 0 {
			common.ApiError(c, errors.New("配置 ID 无效"))
			return
		}
		binding, err = model.GetTypeSafeUsageBinding(id)
		if err != nil {
			common.ApiError(c, err)
			return
		}
	}
	binding.Name, binding.Note, binding.Enabled = strings.TrimSpace(input.Name), strings.TrimSpace(input.Note), input.Enabled
	if binding.Name == "" || len(binding.Name) > 128 {
		common.ApiError(c, errors.New("名称不能为空且不能超过 128 字节"))
		return
	}
	if strings.TrimSpace(input.RequestCurl) != "" {
		binding.RequestCurl = strings.TrimSpace(input.RequestCurl)
	}
	if input.Proxy != nil {
		binding.Proxy = strings.TrimSpace(*input.Proxy)
	}
	if _, err := buildTypeSafeUsageRequest(binding.RequestCurl); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.SaveTypeSafeUsageBinding(binding); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, binding)
}

func DeleteTypeSafeUsageBinding(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiError(c, errors.New("配置 ID 无效"))
		return
	}
	if err := model.DeleteTypeSafeUsageBinding(id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func RefreshTypeSafeUsageBinding(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiError(c, errors.New("配置 ID 无效"))
		return
	}
	binding, err := model.GetTypeSafeUsageBinding(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !binding.Enabled {
		common.ApiError(c, errors.New("TypeSafe 用量配置已禁用"))
		return
	}
	encoded, err := refreshTypeSafeUsage(c.Request.Context(), binding)
	lastError := ""
	if err != nil {
		lastError = err.Error()
	}
	if err := model.UpdateTypeSafeUsageSnapshot(id, encoded, lastError); err != nil {
		common.ApiError(c, err)
		return
	}
	binding, err = model.GetTypeSafeUsageBinding(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, binding)
}

func buildTypeSafeUsageRequest(raw string) (quotaCurlRequest, error) {
	config, err := parseQuotaCurlRequest(raw)
	if err != nil {
		return quotaCurlRequest{}, errors.New("请粘贴 TypeSafe 用量接口的完整 curl 请求")
	}
	u, err := url.Parse(config.URL)
	if err != nil || u.Scheme != "https" || u.Host != "console.typesafe.ai" || u.Path != "/api/usage" || u.User != nil || u.Fragment != "" || config.Method != http.MethodGet || config.Body != "" {
		return quotaCurlRequest{}, errors.New("仅支持 https://console.typesafe.ai/api/usage 的 GET 请求")
	}
	// 小时桶支持最近 31 天；日期和流量筛选在展示层完成。
	config.URL = "https://console.typesafe.ai/api/usage?granularity=hour"
	headers := map[string]string{}
	for key, value := range config.Headers {
		if strings.EqualFold(key, "Cookie") || strings.EqualFold(key, "Authorization") {
			headers[http.CanonicalHeaderKey(key)] = value
		}
	}
	if headers["Cookie"] == "" && headers["Authorization"] == "" {
		return quotaCurlRequest{}, errors.New("TypeSafe curl 缺少控制台登录凭据")
	}
	config.Headers = headers
	return config, nil
}

func refreshTypeSafeUsage(ctx context.Context, binding *model.TypeSafeUsageBinding) (string, error) {
	config, err := buildTypeSafeUsageRequest(binding.RequestCurl)
	if err != nil {
		return "", err
	}
	client, err := quotaHTTPClient(resolveQuotaProxy(binding.Proxy, config.Proxy))
	if err != nil {
		return "", errors.New("TypeSafe 代理配置无效")
	}
	// 禁止重定向，确保登录凭据仅发送至已验证的官方接口。
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, config.URL, nil)
	if err != nil {
		return "", errors.New("无法创建 TypeSafe 请求")
	}
	for key, value := range config.Headers {
		request.Header.Set(key, value)
	}
	request.Header.Set("Accept", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return "", errors.New("TypeSafe 请求失败，请检查网络和代理设置")
	}
	defer response.Body.Close()
	if response.StatusCode == 401 || response.StatusCode == 403 {
		return "", errors.New("TypeSafe 登录已失效，请重新复制用量接口 curl")
	}
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("TypeSafe 返回 HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 8*1024*1024+1))
	if err != nil || len(body) > 8*1024*1024 {
		return "", errors.New("TypeSafe 用量响应读取失败或过大")
	}
	return decodeTypeSafeUsage(body)
}

func decodeTypeSafeUsage(body []byte) (string, error) {
	var data struct {
		Buckets *[]typeSafeUsageBucket `json:"buckets"`
	}
	if err := common.Unmarshal(body, &data); err != nil || data.Buckets == nil {
		return "", errors.New("TypeSafe 用量响应格式无效")
	}
	for _, bucket := range *data.Buckets {
		_, err := time.Parse(time.RFC3339, bucket.Day)
		if err != nil || bucket.Requests < 0 || bucket.InputTokens < 0 || bucket.OutputTokens < 0 || bucket.Requests > 9007199254740991 || bucket.InputTokens > 9007199254740991 || bucket.OutputTokens > 9007199254740991 {
			return "", errors.New("TypeSafe 用量响应包含无效的时间或数量")
		}
	}
	encoded, err := common.Marshal(*data.Buckets)
	return string(encoded), err
}

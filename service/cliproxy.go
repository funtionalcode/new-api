package service

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	cliproxyAPIAuthFilesPath = "/v0/management/auth-files"
	cliproxyAPICallPath      = "/v0/management/api-call"
	cliproxyAPITimeout       = 10 * time.Second
)

type CliproxyAPIClient struct {
	baseURL  string
	password string
	client   *http.Client
}

type CliproxyAuthFile struct {
	AuthIndex string `json:"authIndex"`
	Name      string `json:"name"`
	AuthFile  string `json:"authFile"`
	AccountID string `json:"accountId"`
	Enabled   bool   `json:"enabled"`
}

type cliproxyAuthFilesResponse struct {
	Data  []cliproxyAuthFileResponse `json:"data"`
	Files []cliproxyAuthFileResponse `json:"files"`
}

type cliproxyAuthFileResponse struct {
	AuthIndex      string                `json:"auth_index"`
	CamelAuthIndex string                `json:"authIndex"`
	Name           string                `json:"name"`
	ID             string                `json:"id"`
	AuthFile       string                `json:"authFile"`
	AccountID      string                `json:"account_id"`
	CamelAccountID string                `json:"accountId"`
	IDToken        cliproxyAuthFileToken `json:"id_token"`
	Disabled       bool                  `json:"disabled"`
	Enabled        *bool                 `json:"enabled"`
}

type cliproxyAuthFileToken struct {
	ChatGPTAccountID string `json:"chatgpt_account_id"`
}

type CliproxyAPICallRequest struct {
	AuthIndex string            `json:"authIndex"`
	Method    string            `json:"method"`
	URL       string            `json:"url"`
	Header    map[string]string `json:"header"`
}

type CliproxyAPICallResponse struct {
	Status     int            `json:"status"`
	StatusCode int            `json:"status_code"`
	Body       map[string]any `json:"-"`
	Data       map[string]any `json:"-"`
}

func (response *CliproxyAPICallResponse) UnmarshalJSON(data []byte) error {
	var raw struct {
		Status     int `json:"status"`
		StatusCode int `json:"status_code"`
		Body       any `json:"body"`
		Data       any `json:"data"`
	}
	if err := common.Unmarshal(data, &raw); err != nil {
		return err
	}
	response.Status = raw.Status
	response.StatusCode = raw.StatusCode
	response.Body = parseCliproxyAPICallPayload(raw.Body)
	response.Data = parseCliproxyAPICallPayload(raw.Data)
	return nil
}

func parseCliproxyAPICallPayload(value any) map[string]any {
	switch typedValue := value.(type) {
	case map[string]any:
		return typedValue
	case string:
		var data map[string]any
		if err := common.UnmarshalJsonStr(strings.TrimSpace(typedValue), &data); err != nil {
			return nil
		}
		return data
	default:
		return nil
	}
}

func NewCliproxyAPIClient(baseURL string, password string) (*CliproxyAPIClient, error) {
	parsedURL, err := normalizeCliproxyBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	trimmedPassword := strings.TrimSpace(password)
	if trimmedPassword == "" {
		return nil, fmt.Errorf("cliproxyapi 登录密码不能为空")
	}
	return &CliproxyAPIClient{
		baseURL:  parsedURL,
		password: trimmedPassword,
		client:   &http.Client{Timeout: cliproxyAPITimeout},
	}, nil
}

func (client *CliproxyAPIClient) ListAuthFiles(ctx context.Context) ([]CliproxyAuthFile, error) {
	if client == nil {
		return nil, fmt.Errorf("cliproxyapi 客户端不能为空")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+cliproxyAPIAuthFilesPath, nil)
	if err != nil {
		return nil, fmt.Errorf("创建认证文件请求失败: %w", err)
	}
	client.setAuthHeader(req)

	resp, err := client.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求认证文件列表失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("请求认证文件列表失败，状态码: %d", resp.StatusCode)
	}

	var result cliproxyAuthFilesResponse
	if err := common.DecodeJson(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("解析认证文件列表失败: %w", err)
	}
	return normalizeCliproxyAuthFiles(result), nil
}

func normalizeCliproxyAuthFiles(result cliproxyAuthFilesResponse) []CliproxyAuthFile {
	items := result.Data
	if len(items) == 0 {
		items = result.Files
	}
	files := make([]CliproxyAuthFile, 0, len(items))
	for _, item := range items {
		files = append(files, CliproxyAuthFile{
			AuthIndex: firstNonEmpty(item.AuthIndex, item.CamelAuthIndex),
			Name:      item.Name,
			AuthFile:  firstNonEmpty(item.ID, item.AuthFile),
			AccountID: firstNonEmpty(item.AccountID, item.CamelAccountID, item.IDToken.ChatGPTAccountID),
			Enabled:   item.Enabled == nil && !item.Disabled || item.Enabled != nil && *item.Enabled,
		})
	}
	return files
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (client *CliproxyAPIClient) CallAPI(ctx context.Context, payload CliproxyAPICallRequest) (*CliproxyAPICallResponse, error) {
	if client == nil {
		return nil, fmt.Errorf("cliproxyapi 客户端不能为空")
	}
	if strings.TrimSpace(payload.AuthIndex) == "" {
		return nil, fmt.Errorf("认证文件索引不能为空")
	}
	if strings.TrimSpace(payload.Method) == "" {
		return nil, fmt.Errorf("请求方法不能为空")
	}
	if strings.TrimSpace(payload.URL) == "" {
		return nil, fmt.Errorf("请求地址不能为空")
	}

	body, err := common.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("序列化刷新请求失败: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, client.baseURL+cliproxyAPICallPath, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建刷新请求失败: %w", err)
	}
	client.setAuthHeader(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("刷新额度失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("刷新额度失败，状态码: %d", resp.StatusCode)
	}

	var result CliproxyAPICallResponse
	if err := common.DecodeJson(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("解析刷新结果失败: %w", err)
	}
	normalizeCliproxyAPICallResponse(&result)
	if result.Status >= http.StatusBadRequest {
		return nil, fmt.Errorf("刷新额度失败，上游状态码: %d", result.Status)
	}
	return &result, nil
}

func normalizeCliproxyAPICallResponse(result *CliproxyAPICallResponse) {
	if result.Status == 0 && result.StatusCode > 0 {
		result.Status = result.StatusCode
	}
	if len(result.Body) == 0 && len(result.Data) > 0 {
		result.Body = result.Data
	}
}

func (client *CliproxyAPIClient) setAuthHeader(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+client.password)
	req.Header.Set("Accept", "application/json")
}

func normalizeCliproxyBaseURL(rawURL string) (string, error) {
	trimmedURL := strings.TrimRight(strings.TrimSpace(rawURL), "/")
	if trimmedURL == "" {
		return "", fmt.Errorf("cliproxyapi 地址不能为空")
	}
	parsedURL, err := url.Parse(trimmedURL)
	if err != nil {
		return "", fmt.Errorf("cliproxyapi 地址无效: %w", err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return "", fmt.Errorf("cliproxyapi 地址仅支持 http 或 https")
	}
	if parsedURL.Host == "" {
		return "", fmt.Errorf("cliproxyapi 地址缺少主机")
	}
	parsedURL.Path = strings.TrimRight(parsedURL.Path, "/")
	parsedURL.RawQuery = ""
	parsedURL.Fragment = ""
	return parsedURL.String(), nil
}

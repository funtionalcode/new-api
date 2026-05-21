package codexchat

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const (
	cfClearanceWarmupPath = "/backend-api/wham/usage"
	cfClearanceTTL        = 30 * time.Minute
	cfClearanceBodyLimit  = 4096
)

type cfClearanceEntry struct {
	Cookie    string
	ExpiresAt time.Time
	Proxy     string
}

var cfClearanceStore sync.Map // key: channelId (int), value: *cfClearanceEntry

// isCloudflareChallenge 检测 Cloudflare 挑战响应
func isCloudflareChallenge(resp *http.Response) bool {
	if resp == nil {
		return false
	}
	// 检测 403 状态码 + HTML 内容
	if resp.StatusCode == http.StatusForbidden {
		body, err := io.ReadAll(io.LimitReader(resp.Body, cfClearanceBodyLimit))
		if err != nil {
			return false
		}
		// 恢复 body 供后续读取
		resp.Body = io.NopCloser(bytes.NewReader(body))
		lower := strings.ToLower(string(body))
		return strings.Contains(lower, "cf_chl_opt") ||
			strings.Contains(lower, "cf-chl-widget") ||
			strings.Contains(lower, "challenge-platform") ||
			strings.Contains(lower, "just a moment") ||
			strings.Contains(lower, "cf-browser-verification")
	}
	// 检测 503 + Server: cloudflare
	if resp.StatusCode == http.StatusServiceUnavailable {
		if strings.Contains(strings.ToLower(resp.Header.Get("Server")), "cloudflare") {
			return true
		}
	}
	return false
}

// getCfClearance 从缓存获取有效的 cf_clearance cookie
func getCfClearance(channelId int, currentProxy string) (string, bool) {
	val, ok := cfClearanceStore.Load(channelId)
	if !ok {
		return "", false
	}
	entry, ok := val.(*cfClearanceEntry)
	if !ok {
		return "", false
	}
	if time.Now().After(entry.ExpiresAt) {
		cfClearanceStore.Delete(channelId)
		return "", false
	}
	if entry.Proxy != currentProxy {
		cfClearanceStore.Delete(channelId)
		return "", false
	}
	return entry.Cookie, true
}

// invalidateCfClearance 失效缓存的 cookie
func invalidateCfClearance(channelId int) {
	cfClearanceStore.Delete(channelId)
}

// warmupCfClearance 通过 warmup 请求获取 cf_clearance cookie
func warmupCfClearance(c *gin.Context, channelId int, baseURL string, apiKey string, proxy string) (string, error) {
	bu := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if bu == "" {
		return "", fmt.Errorf("empty baseURL")
	}

	client := service.GetHttpClient()
	if proxy != "" {
		var err error
		client, err = service.NewProxyHttpClient(proxy)
		if err != nil {
			return "", fmt.Errorf("create proxy client failed: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, bu+cfClearanceWarmupPath, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("originator", "codex_cli_rs")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("warmup request failed: %w", err)
	}
	defer resp.Body.Close()
	// 读取并丢弃 body
	io.ReadAll(resp.Body)

	// 从 Set-Cookie 中提取 cf_clearance
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "cf_clearance" && cookie.Value != "" {
			// 存入缓存
			cfClearanceStore.Store(channelId, &cfClearanceEntry{
				Cookie:    cookie.Value,
				ExpiresAt: time.Now().Add(cfClearanceTTL),
				Proxy:     proxy,
			})
			logger.LogDebug(c, "codexchat: acquired cf_clearance for channel %d", channelId)
			return cookie.Value, nil
		}
	}

	return "", fmt.Errorf("no cf_clearance cookie in warmup response (status=%d)", resp.StatusCode)
}

// injectCfClearance 向请求注入 cf_clearance cookie
func injectCfClearance(req *http.Request, cookie string) {
	if cookie == "" {
		return
	}
	req.Header.Set("Cookie", "cf_clearance="+cookie)
}

// ensureCfClearance 确保有有效的 cf_clearance cookie
func ensureCfClearance(c *gin.Context, channelId int, baseURL string, apiKey string, proxy string) string {
	cookie, ok := getCfClearance(channelId, proxy)
	if ok {
		return cookie
	}
	cookie, err := warmupCfClearance(c, channelId, baseURL, apiKey, proxy)
	if err != nil {
		logger.LogWarn(c, fmt.Sprintf("codexchat: warmup cf_clearance failed: %v", err))
		return ""
	}
	return cookie
}

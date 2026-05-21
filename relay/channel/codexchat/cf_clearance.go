package codexchat

import (
	"bytes"
	"io"
	"net/http"
	"strings"
)

const cfClearanceBodyLimit = 4096

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

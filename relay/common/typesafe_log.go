package common

import "unicode/utf8"

// TypeSafeLogBody 保存实际请求或响应的有限快照，不包含 HTTP 头。
type TypeSafeLogBody struct {
	Body      string `json:"body"`
	Bytes     int    `json:"bytes"`
	Truncated bool   `json:"truncated,omitempty"`
}

type TypeSafeExchange struct {
	Request  *TypeSafeLogBody `json:"request,omitempty"`
	Response *TypeSafeLogBody `json:"response,omitempty"`
}

// CaptureTypeSafeLogBody 限制每个方向的日志体积，并保留完整 UTF-8 字符。
func CaptureTypeSafeLogBody(data []byte) *TypeSafeLogBody {
	const maxBytes = 128 * 1024
	end := len(data)
	if end > maxBytes {
		end = maxBytes
		for end > 0 && !utf8.RuneStart(data[end]) {
			end--
		}
	}
	return &TypeSafeLogBody{Body: string(data[:end]), Bytes: len(data), Truncated: end < len(data)}
}

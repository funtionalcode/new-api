package controller

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"unicode"
)

type quotaCurlRequest struct {
	Method  string
	URL     string
	Headers map[string]string
	Proxy   string
}

func parseQuotaCurlRequest(rawCurl string) (quotaCurlRequest, error) {
	tokens, err := splitQuotaCurlCommand(rawCurl)
	if err != nil {
		return quotaCurlRequest{}, err
	}
	if len(tokens) == 0 || strings.ToLower(tokens[0]) != "curl" {
		return quotaCurlRequest{}, fmt.Errorf("请输入完整的 curl 命令")
	}
	requestConfig := quotaCurlRequest{
		Method:  http.MethodGet,
		Headers: map[string]string{},
	}
	for index := 1; index < len(tokens); index++ {
		token := tokens[index]
		switch token {
		case "-H", "--header":
			index++
			if index >= len(tokens) {
				return quotaCurlRequest{}, fmt.Errorf("curl header 参数缺少值")
			}
			addQuotaCurlHeader(requestConfig.Headers, tokens[index])
		case "-b", "--cookie":
			index++
			if index >= len(tokens) {
				return quotaCurlRequest{}, fmt.Errorf("curl cookie 参数缺少值")
			}
			appendQuotaCurlCookie(requestConfig.Headers, tokens[index])
		case "-X", "--request":
			index++
			if index >= len(tokens) {
				return quotaCurlRequest{}, fmt.Errorf("curl request 参数缺少值")
			}
			requestConfig.Method = strings.ToUpper(strings.TrimSpace(tokens[index]))
		case "--url":
			index++
			if index >= len(tokens) {
				return quotaCurlRequest{}, fmt.Errorf("curl url 参数缺少值")
			}
			requestConfig.URL = tokens[index]
		case "-x", "--proxy":
			index++
			if index >= len(tokens) {
				return quotaCurlRequest{}, fmt.Errorf("curl proxy 参数缺少值")
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
		return quotaCurlRequest{}, fmt.Errorf("curl 中未找到请求地址")
	}
	parsedURL, err := url.Parse(requestConfig.URL)
	if err != nil {
		return quotaCurlRequest{}, err
	}
	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return quotaCurlRequest{}, fmt.Errorf("curl 请求地址无效")
	}
	requestConfig.URL = parsedURL.String()
	return requestConfig, nil
}

func splitQuotaCurlCommand(rawCurl string) ([]string, error) {
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

func addQuotaCurlHeader(headers map[string]string, rawHeader string) {
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
		appendQuotaCurlCookie(headers, value)
		return
	}
	headers[key] = value
}

func appendQuotaCurlCookie(headers map[string]string, cookie string) {
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

func shouldSkipQuotaCurlHeader(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "content-length", "connection":
		return true
	default:
		return false
	}
}

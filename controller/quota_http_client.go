package controller

import (
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/service"
)

func quotaHTTPClient(proxyURL string) (*http.Client, error) {
	proxyURL = strings.TrimSpace(proxyURL)
	if proxyURL == "" {
		return &http.Client{Timeout: 30 * time.Second}, nil
	}
	client, err := service.GetHttpClientWithProxy(proxyURL)
	if err != nil {
		return nil, err
	}
	if client == nil {
		return &http.Client{Timeout: 30 * time.Second}, nil
	}
	clientCopy := *client
	clientCopy.Timeout = 30 * time.Second
	return &clientCopy, nil
}

func resolveQuotaProxy(configuredProxy string, curlProxy string) string {
	if proxy := strings.TrimSpace(configuredProxy); proxy != "" {
		return proxy
	}
	return strings.TrimSpace(curlProxy)
}

func quotaCurlInlineProxy(token string) (string, bool) {
	token = strings.TrimSpace(token)
	if strings.HasPrefix(token, "--proxy=") {
		return strings.TrimSpace(strings.TrimPrefix(token, "--proxy=")), true
	}
	if strings.HasPrefix(token, "-x") && len(token) > len("-x") {
		return strings.TrimSpace(strings.TrimPrefix(token, "-x")), true
	}
	return "", false
}

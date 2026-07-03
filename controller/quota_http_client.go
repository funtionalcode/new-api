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

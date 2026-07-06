package controller

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestQuotaHTTPClientUsesDefaultTimeoutWithoutProxy(t *testing.T) {
	client, err := quotaHTTPClient("")
	if err != nil {
		t.Fatalf("quotaHTTPClient returned error: %v", err)
	}
	if client == nil {
		t.Fatal("client is nil")
	}
	if client.Timeout != 30*time.Second {
		t.Fatalf("Timeout = %s, want 30s", client.Timeout)
	}
}

func TestQuotaHTTPClientRejectsUnsupportedProxyScheme(t *testing.T) {
	_, err := quotaHTTPClient("ftp://example.com:21")
	if err == nil {
		t.Fatal("quotaHTTPClient returned nil error for unsupported proxy")
	}
}

func TestQuotaProxyLabelRedactsPassword(t *testing.T) {
	label := quotaProxyLabel("socks5://user:secret@example.com:1080")
	if strings.Contains(label, "secret") {
		t.Fatalf("quotaProxyLabel leaked password: %s", label)
	}
	if label != "socks5://user:xxxxx@example.com:1080" {
		t.Fatalf("quotaProxyLabel = %q", label)
	}
}

func TestQuotaHTTPRequestErrorMentionsRedactedProxy(t *testing.T) {
	sentinel := errors.New("boom")
	err := quotaHTTPRequestError("socks5://user:secret@example.com:1080", sentinel)
	if !errors.Is(err, sentinel) {
		t.Fatalf("quotaHTTPRequestError does not wrap original error: %v", err)
	}
	message := err.Error()
	if !strings.Contains(message, "经代理 socks5://user:xxxxx@example.com:1080 请求失败") {
		t.Fatalf("error message = %q", message)
	}
	if strings.Contains(message, "secret") {
		t.Fatalf("quotaHTTPRequestError leaked password: %s", message)
	}
}

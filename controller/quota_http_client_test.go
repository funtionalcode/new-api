package controller

import (
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

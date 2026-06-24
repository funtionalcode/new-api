package common

import (
	"net/http"
	"testing"
)

func withTrustedProxyCIDRs(t *testing.T, value string) {
	t.Helper()
	old := TrustedProxyCIDRs
	TrustedProxyCIDRs = value
	t.Cleanup(func() {
		TrustedProxyCIDRs = old
	})
}

func TestGetRequestClientIPUsesForwardedForFromTrustedProxy(t *testing.T) {
	withTrustedProxyCIDRs(t, "172.16.0.0/12")
	req := &http.Request{
		RemoteAddr: "172.22.0.1:51234",
		Header: http.Header{
			"X-Forwarded-For": {"198.51.100.10, 172.22.0.1"},
			"X-Real-Ip":       {"203.0.113.20"},
		},
	}

	if got := GetRequestClientIP(req); got != "198.51.100.10" {
		t.Fatalf("GetRequestClientIP() = %q, want %q", got, "198.51.100.10")
	}
}

func TestGetRequestClientIPIgnoresForwardedForFromUntrustedClient(t *testing.T) {
	withTrustedProxyCIDRs(t, "172.16.0.0/12")
	req := &http.Request{
		RemoteAddr: "203.0.113.8:51234",
		Header: http.Header{
			"X-Forwarded-For": {"198.51.100.10"},
			"X-Real-Ip":       {"198.51.100.11"},
		},
	}

	if got := GetRequestClientIP(req); got != "203.0.113.8" {
		t.Fatalf("GetRequestClientIP() = %q, want %q", got, "203.0.113.8")
	}
}

func TestGetRequestClientIPFallsBackToRealIPWhenForwardedForInvalid(t *testing.T) {
	withTrustedProxyCIDRs(t, "172.16.0.0/12")
	req := &http.Request{
		RemoteAddr: "172.22.0.1:51234",
		Header: http.Header{
			"X-Forwarded-For": {"unknown, garbage"},
			"X-Real-Ip":       {"198.51.100.11"},
		},
	}

	if got := GetRequestClientIP(req); got != "198.51.100.11" {
		t.Fatalf("GetRequestClientIP() = %q, want %q", got, "198.51.100.11")
	}
}

func TestGetRequestClientIPUsesRemoteAddrWhenTrustedProxyHeadersMissing(t *testing.T) {
	withTrustedProxyCIDRs(t, "172.16.0.0/12")
	req := &http.Request{RemoteAddr: "172.22.0.1:51234"}

	if got := GetRequestClientIP(req); got != "172.22.0.1" {
		t.Fatalf("GetRequestClientIP() = %q, want %q", got, "172.22.0.1")
	}
}

func TestGetRequestClientIPDisablesHeaderTrustWhenProxyCIDRsEmpty(t *testing.T) {
	withTrustedProxyCIDRs(t, "")
	req := &http.Request{
		RemoteAddr: "172.22.0.1:51234",
		Header: http.Header{
			"X-Forwarded-For": {"198.51.100.10"},
		},
	}

	if got := GetRequestClientIP(req); got != "172.22.0.1" {
		t.Fatalf("GetRequestClientIP() = %q, want %q", got, "172.22.0.1")
	}
}

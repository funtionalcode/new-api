package common

import (
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func GetClientIP(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if ip := GetRequestClientIP(c.Request); ip != "" {
		return ip
	}
	// Fallback for edge cases where RemoteAddr is missing/unparseable.
	return c.ClientIP()
}

func GetRequestClientIP(req *http.Request) string {
	if req == nil {
		return ""
	}

	remoteIP := parseClientIPValue(req.RemoteAddr)
	if remoteIP == nil {
		return ""
	}

	if !isTrustedProxy(remoteIP) {
		return remoteIP.String()
	}

	if ip := firstForwardedForIP(req.Header.Get("X-Forwarded-For")); ip != nil {
		return ip.String()
	}
	if ip := parseClientIPValue(req.Header.Get("X-Real-IP")); ip != nil {
		return ip.String()
	}
	return remoteIP.String()
}

func firstForwardedForIP(header string) net.IP {
	for _, part := range strings.Split(header, ",") {
		if ip := parseClientIPValue(part); ip != nil {
			return ip
		}
	}
	return nil
}

func parseClientIPValue(value string) net.IP {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"`)
	if value == "" || strings.EqualFold(value, "unknown") {
		return nil
	}

	if host, _, err := net.SplitHostPort(value); err == nil {
		value = host
	} else if strings.HasPrefix(value, "[") && strings.Contains(value, "]") {
		value = strings.TrimPrefix(value, "[")
		if idx := strings.LastIndex(value, "]"); idx >= 0 {
			value = value[:idx]
		}
	}

	return net.ParseIP(strings.TrimSpace(value))
}

func isTrustedProxy(ip net.IP) bool {
	cidrs := splitTrustedProxyCIDRs(TrustedProxyCIDRs)
	if len(cidrs) == 0 {
		return false
	}
	return IsIpInCIDRList(ip, cidrs)
}

func splitTrustedProxyCIDRs(value string) []string {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	})
	cidrs := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field != "" {
			cidrs = append(cidrs, field)
		}
	}
	return cidrs
}

package kitutil

import (
	"maps"
	"net"
	"net/url"
	"regexp"
	"slices"
	"strings"
)

var (
	// maskURLPattern stops at characters a URL cannot contain unescaped, such
	// as double quotes and angle or curly brackets, so a URL quoted in an error
	// keeps its delimiters. Apostrophes and parentheses are valid inside a URL;
	// the caller trims them, like other punctuation, only when they trail it.
	maskURLPattern = regexp.MustCompile(`(?:https?|wss?)://[^\s"<>{}]+`)
	maskIPPattern  = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	// maskBracketedIPv6Pattern matches the bracketed form Go network errors use
	// for IPv6 addresses, as in "dial tcp [2001:db8::1]:443", with an optional
	// zone; the caller confirms the content parses as an IP.
	maskBracketedIPv6Pattern = regexp.MustCompile(`\[([0-9A-Fa-f:.]*:[0-9A-Fa-f:.]*)(%[^\]\s]*)?\]`)
	// maskApiKeyPattern matches patterns like 'api_key:xxx' or "api_key:xxx" to mask the API key value
	maskApiKeyPattern = regexp.MustCompile(`(['"]?)api_key:([^\s'"]+)(['"]?)`)
)

// maskURLQuery keeps the parameter names of a raw query and masks the values.
// Example: key=secret&alt=sse -> ?alt=***&key=***
func maskURLQuery(rawQuery string) string {
	if rawQuery == "" {
		return ""
	}
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		// If can't parse query, just mask the whole query string
		return "?***"
	}
	if len(values) == 0 {
		return ""
	}
	maskedParams := make([]string, 0, len(values))
	for _, key := range slices.Sorted(maps.Keys(values)) {
		maskedParams = append(maskedParams, key+"=***")
	}
	return "?" + strings.Join(maskedParams, "&")
}

// MaskSensitiveInfo 保留域名用于排障，仅隐藏指定上游的路径。
// 所有 URL 的查询值、用户凭据及 IPv4/IPv6 地址始终脱敏。
func MaskSensitiveInfo(str string, upstreamURLs ...string) string {
	// Mask API keys (e.g., "api_key:AIzaSyAAAaUooTUni8AdaOkSRMda30n_Q4vrV70" -> "api_key:***")
	str = maskApiKeyPattern.ReplaceAllString(str, "${1}api_key:***${3}")

	upstreamHosts := make([]string, 0, len(upstreamURLs))
	for _, upstreamURL := range upstreamURLs {
		upstreamURL = strings.TrimSpace(upstreamURL)
		if upstreamURL == "" {
			continue
		}
		if !strings.Contains(upstreamURL, "://") {
			// A base URL saved without a scheme would otherwise parse as a path.
			upstreamURL = "//" + upstreamURL
		}
		parsed, err := url.Parse(upstreamURL)
		if err != nil {
			continue
		}
		// A fully qualified host keeps its trailing root dot; errors may omit it.
		if host := strings.TrimSuffix(parsed.Hostname(), "."); host != "" {
			upstreamHosts = append(upstreamHosts, host)
		}
	}
	// Mask URLs on an upstream host, and the query values of every other URL
	str = maskURLPattern.ReplaceAllStringFunc(str, func(match string) string {
		urlStr := match
		for urlStr != "" {
			last := urlStr[len(urlStr)-1]
			if strings.IndexByte(".,;:!?'", last) < 0 && (last != ')' || strings.Count(urlStr, "(") >= strings.Count(urlStr, ")")) {
				break
			}
			urlStr = urlStr[:len(urlStr)-1]
		}
		suffix := match[len(urlStr):]
		u, err := url.Parse(urlStr)
		if err != nil {
			return match
		}
		host := strings.TrimSuffix(u.Hostname(), ".")
		if !slices.ContainsFunc(upstreamHosts, func(upstreamHost string) bool { return strings.EqualFold(upstreamHost, host) }) {
			if u.RawQuery == "" && u.User == nil {
				return match
			}
			// Userinfo is a credential wherever the URL points.
			if u.User != nil {
				scheme, afterScheme, _ := strings.Cut(urlStr, "://")
				authorityEnd := strings.IndexAny(afterScheme, "/?#")
				if authorityEnd < 0 {
					authorityEnd = len(afterScheme)
				}
				urlStr = scheme + "://***@" + afterScheme[strings.LastIndex(afterScheme[:authorityEnd], "@")+1:]
			}
			if u.RawQuery == "" {
				return urlStr + suffix
			}
			beforeQuery, _, _ := strings.Cut(urlStr, "?")
			fragment := ""
			if u.Fragment != "" {
				fragment = "#" + u.EscapedFragment()
			}
			return beforeQuery + maskURLQuery(u.RawQuery) + fragment + suffix
		}

		maskedHost := host
		if ip := net.ParseIP(host); ip != nil {
			maskedHost = "***"
			if ip.To4() != nil {
				maskedHost = "***.***.***.***"
			}
		}
		if port := u.Port(); port != "" {
			maskedHost += ":" + port
		}

		result := u.Scheme + "://" + maskedHost

		// Mask path
		if u.Path != "" && u.Path != "/" {
			pathParts := strings.Split(strings.Trim(u.Path, "/"), "/")
			maskedPathParts := make([]string, len(pathParts))
			for i := range pathParts {
				if pathParts[i] != "" {
					maskedPathParts[i] = "***"
				}
			}
			if len(maskedPathParts) > 0 {
				result += "/" + strings.Join(maskedPathParts, "/")
			}
		} else if u.Path == "/" {
			result += "/"
		}

		return result + maskURLQuery(u.RawQuery) + suffix
	})

	// Mask IP addresses: in network errors they are the resolved upstream
	// address or the gateway's own, so they are masked even without an
	// upstream address, and an IP upstream host is covered too. IPv6 goes
	// first so an embedded IPv4 such as [::ffff:10.0.0.1] still parses.
	str = maskBracketedIPv6Pattern.ReplaceAllStringFunc(str, func(match string) string {
		if net.ParseIP(maskBracketedIPv6Pattern.FindStringSubmatch(match)[1]) == nil {
			return match
		}
		return "[***]"
	})
	str = maskIPPattern.ReplaceAllString(str, "***.***.***.***")

	return str
}

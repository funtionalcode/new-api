package common

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
)

func TestTypeSafeLogBodyPreservesJSONAndBoundsUnicode(t *testing.T) {
	for _, tc := range []struct {
		name      string
		body      string
		want      string
		truncated bool
	}{
		{"完整请求", `{"state":"你好","questions":{}}`, `{"state":"你好","questions":{}}`, false},
		{"超长内容", strings.Repeat("a", 128*1024-1) + "你还有内容", strings.Repeat("a", 128*1024-1), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := CaptureTypeSafeLogBody([]byte(tc.body))
			assert.Equal(t, tc.want, result.Body)
			assert.Equal(t, len(tc.body), result.Bytes)
			assert.Equal(t, tc.truncated, result.Truncated)
			assert.True(t, utf8.ValidString(result.Body))
		})
	}
}

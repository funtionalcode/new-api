package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTypeSafeCapturePreservesStreamAndCollectsOnlyAnswer(t *testing.T) {
	capture := &TypeSafeCapture{Limit: 20}
	capture.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"你好\",\"reasoning_content\":\"private\"}}]}\n\ndata: {\"type\":\"response.output_text."), true)
	capture.Write([]byte("delta\",\"delta\":\"世界\"}\n\ndata: [DONE]\n\n"), true)
	assert.Equal(t, "你好世界", capture.Text())
	assert.False(t, capture.Truncated)
	capture.ObserveJSON([]byte(`{"type":"response.completed","response":{"output":[{"type":"message","content":[{"text":"你好世界"}]}]}}`))
	assert.Equal(t, "你好世界", capture.Text())
}

func TestTypeSafeCaptureBoundsAndFormats(t *testing.T) {
	for _, body := range []string{
		`{"choices":[{"message":{"content":"hello world"}}]}`,
		`{"content":[{"type":"thinking","thinking":"private"},{"type":"text","text":"hello world"}]}`,
		`{"candidates":[{"content":{"parts":[{"text":"private","thought":true},{"text":"hello world"}]}}]}`,
	} {
		capture := &TypeSafeCapture{Limit: 5}
		capture.Write([]byte(body), false)
		assert.Equal(t, "hello", capture.Text())
		assert.True(t, capture.Truncated)
	}
}

func TestTypeSafeLocalParametersNeverReachUpstream(t *testing.T) {
	for _, config := range []map[string]any{
		{"_typesafe": map[string]any{"channel_id": 66}},
		{"_typesafe": map[string]any{"channel_id": 66}, "temperature": 0.5},
		{"_typesafe": map[string]any{"channel_id": 66}, "operations": []any{map[string]any{"path": "temperature", "mode": "set", "value": 0.5}}},
	} {
		result, err := ApplyParamOverride([]byte(`{"model":"test"}`), config, nil)
		require.NoError(t, err)
		assert.NotContains(t, string(result), "typesafe")
		assert.Contains(t, string(result), `"model":"test"`)
		assert.Contains(t, config, "_typesafe")
	}
}

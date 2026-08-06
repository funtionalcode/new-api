package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsClaudeDesktopClaudeTokenCountProbe(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{
			name: "exact legacy probe",
			body: `{"model":"claude-test","messages":[{"role":"user","content":"count"}],"max_tokens":1,"tools":[{"name":"lookup","input_schema":{"type":"object"}}]}`,
			want: true,
		},
		{
			name: "normal count prompt without tools",
			body: `{"model":"claude-test","messages":[{"role":"user","content":"count"}],"max_tokens":1}`,
			want: false,
		},
		{
			name: "different output limit",
			body: `{"model":"claude-test","messages":[{"role":"user","content":"count"}],"max_tokens":2,"tools":[{"name":"lookup","input_schema":{"type":"object"}}]}`,
			want: false,
		},
		{
			name: "different prompt",
			body: `{"model":"claude-test","messages":[{"role":"user","content":"Count"}],"max_tokens":1,"tools":[{"name":"lookup","input_schema":{"type":"object"}}]}`,
			want: false,
		},
		{
			name: "streaming request",
			body: `{"model":"claude-test","messages":[{"role":"user","content":"count"}],"max_tokens":1,"stream":true,"tools":[{"name":"lookup","input_schema":{"type":"object"}}]}`,
			want: false,
		},
		{
			name: "content array",
			body: `{"model":"claude-test","messages":[{"role":"user","content":[{"type":"text","text":"count"}]}],"max_tokens":1,"tools":[{"name":"lookup","input_schema":{"type":"object"}}]}`,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var request dto.ClaudeRequest
			require.NoError(t, common.Unmarshal([]byte(tt.body), &request))
			assert.Equal(t, tt.want, isClaudeDesktopClaudeTokenCountProbe(&request))
		})
	}
}

func TestIsClaudeDesktopOpenAITokenCountProbe(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{
			name: "exact converted probe",
			body: `{"model":"grok-test","messages":[{"role":"user","content":"count"}],"max_tokens":1,"tools":[{"type":"function","function":{"name":"lookup","parameters":{"type":"object"}}}]}`,
			want: true,
		},
		{
			name: "normal count prompt without tools",
			body: `{"model":"grok-test","messages":[{"role":"user","content":"count"}],"max_tokens":1}`,
			want: false,
		},
		{
			name: "different output limit",
			body: `{"model":"grok-test","messages":[{"role":"user","content":"count"}],"max_tokens":2,"tools":[{"type":"function","function":{"name":"lookup","parameters":{"type":"object"}}}]}`,
			want: false,
		},
		{
			name: "different prompt",
			body: `{"model":"grok-test","messages":[{"role":"user","content":"Count"}],"max_tokens":1,"tools":[{"type":"function","function":{"name":"lookup","parameters":{"type":"object"}}}]}`,
			want: false,
		},
		{
			name: "streaming request",
			body: `{"model":"grok-test","messages":[{"role":"user","content":"count"}],"max_tokens":1,"stream":true,"tools":[{"type":"function","function":{"name":"lookup","parameters":{"type":"object"}}}]}`,
			want: false,
		},
		{
			name: "multiple messages",
			body: `{"model":"grok-test","messages":[{"role":"system","content":"reply briefly"},{"role":"user","content":"count"}],"max_tokens":1,"tools":[{"type":"function","function":{"name":"lookup","parameters":{"type":"object"}}}]}`,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var request dto.GeneralOpenAIRequest
			require.NoError(t, common.Unmarshal([]byte(tt.body), &request))
			assert.Equal(t, tt.want, isClaudeDesktopOpenAITokenCountProbe(&request))
		})
	}
}

func TestIsClaudeDesktopResponsesTokenCountProbe(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{
			name: "exact string input probe",
			body: `{"model":"grok-test","input":"count","max_output_tokens":1,"tools":[{"type":"function","name":"lookup","parameters":{"type":"object"}}]}`,
			want: true,
		},
		{
			name: "exact message input probe",
			body: `{"model":"grok-test","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"count"}]}],"max_output_tokens":1,"tools":[{"type":"function","name":"lookup","parameters":{"type":"object"}}]}`,
			want: true,
		},
		{
			name: "normal count prompt without tools",
			body: `{"model":"grok-test","input":"count","max_output_tokens":1}`,
			want: false,
		},
		{
			name: "different output limit",
			body: `{"model":"grok-test","input":"count","max_output_tokens":2,"tools":[{"type":"function","name":"lookup","parameters":{"type":"object"}}]}`,
			want: false,
		},
		{
			name: "different prompt",
			body: `{"model":"grok-test","input":"Count","max_output_tokens":1,"tools":[{"type":"function","name":"lookup","parameters":{"type":"object"}}]}`,
			want: false,
		},
		{
			name: "streaming request",
			body: `{"model":"grok-test","input":"count","max_output_tokens":1,"stream":true,"tools":[{"type":"function","name":"lookup","parameters":{"type":"object"}}]}`,
			want: false,
		},
		{
			name: "assistant message",
			body: `{"model":"grok-test","input":[{"type":"message","role":"assistant","content":"count"}],"max_output_tokens":1,"tools":[{"type":"function","name":"lookup","parameters":{"type":"object"}}]}`,
			want: false,
		},
		{
			name: "multiple input items",
			body: `{"model":"grok-test","input":[{"role":"user","content":"context"},{"role":"user","content":"count"}],"max_output_tokens":1,"tools":[{"type":"function","name":"lookup","parameters":{"type":"object"}}]}`,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var request dto.OpenAIResponsesRequest
			require.NoError(t, common.Unmarshal([]byte(tt.body), &request))
			assert.Equal(t, tt.want, isClaudeDesktopResponsesTokenCountProbe(&request))
		})
	}
}

package constant

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPath2RelayMode(t *testing.T) {
	tests := []struct {
		name string
		path string
		want int
	}{
		{
			name: "playground images generations",
			path: "/pg/images/generations",
			want: RelayModeImagesGenerations,
		},
		{
			name: "playground chat completions",
			path: "/pg/chat/completions",
			want: RelayModeChatCompletions,
		},
		{
			name: "playground audio speech",
			path: "/pg/audio/speech",
			want: RelayModeAudioSpeech,
		},
		{
			name: "playground audio transcriptions",
			path: "/pg/audio/transcriptions",
			want: RelayModeAudioTranscription,
		},
		{
			name: "v1 images generations",
			path: "/v1/images/generations",
			want: RelayModeImagesGenerations,
		},
		{
			name: "alpha search",
			path: "/v1/alpha/search",
			want: RelayModeAlphaSearch,
		},
		{
			name: "alpha search with query",
			path: "/v1/alpha/search?foo=1",
			want: RelayModeAlphaSearch,
		},
		{
			name: "claude count tokens",
			path: "/v1/messages/count_tokens",
			want: RelayModeClaudeCountTokens,
		},
		{
			name: "claude count tokens with query",
			path: "/v1/messages/count_tokens?beta=true",
			want: RelayModeClaudeCountTokens,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Path2RelayMode(tt.path))
		})
	}
}

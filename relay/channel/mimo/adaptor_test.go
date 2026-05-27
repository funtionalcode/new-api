package mimo

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetRequestURLForAudioSpeech(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		baseURL string
		want    string
	}{
		{
			name:    "root base url",
			baseURL: "https://api.xiaomimimo.com",
			want:    "https://api.xiaomimimo.com/v1/chat/completions",
		},
		{
			name:    "v1 base url",
			baseURL: "https://api.xiaomimimo.com/v1",
			want:    "https://api.xiaomimimo.com/v1/chat/completions",
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			adaptor := &Adaptor{}
			info := &relaycommon.RelayInfo{
				RelayMode: relayconstant.RelayModeAudioSpeech,
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelBaseUrl: testCase.baseURL,
				},
			}

			got, err := adaptor.GetRequestURL(info)

			require.NoError(t, err)
			require.Equal(t, testCase.want, got)
		})
	}
}

func TestConvertAudioRequest(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeAudioSpeech,
		OriginModelName: "mimo-v2.5-tts",
	}
	request := dto.AudioRequest{
		Input:          "Welcome to MiMo.",
		Voice:          "Mia",
		Instructions:   "Read with a warm tone",
		ResponseFormat: "wav",
	}

	reader, err := adaptor.ConvertAudioRequest(ctx, info, request)

	require.NoError(t, err)
	body, err := io.ReadAll(reader)
	require.NoError(t, err)

	var payload ttsRequest
	require.NoError(t, common.Unmarshal(body, &payload))
	require.Equal(t, "mimo-v2.5-tts", payload.Model)
	require.Equal(t, []ttsMessage{
		{Role: "user", Content: "Read with a warm tone"},
		{Role: "assistant", Content: "Welcome to MiMo."},
	}, payload.Messages)
	require.Equal(t, "wav", payload.Audio.Format)
	require.Equal(t, "Mia", payload.Audio.Voice)

	responseFormat, ok := ctx.Get("response_format")
	require.True(t, ok)
	require.Equal(t, "wav", responseFormat)
}

func TestConvertAudioRequestDefaultsOpenAIVoiceToMimoDefault(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeAudioSpeech,
		OriginModelName: "mimo-v2.5-tts",
	}
	request := dto.AudioRequest{
		Input: "Welcome to MiMo.",
		Voice: "alloy",
	}

	reader, err := adaptor.ConvertAudioRequest(ctx, info, request)

	require.NoError(t, err)
	body, err := io.ReadAll(reader)
	require.NoError(t, err)

	var payload ttsRequest
	require.NoError(t, common.Unmarshal(body, &payload))
	require.Equal(t, "mimo_default", payload.Audio.Voice)
}

func TestConvertAudioRequestDefaultsFormatAndOmitsVoicedesignVoice(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeAudioSpeech,
		OriginModelName: "mimo-v2.5-tts-voicedesign",
	}
	request := dto.AudioRequest{
		Input: "Design a new voice",
		Voice: "Mia",
	}

	reader, err := adaptor.ConvertAudioRequest(ctx, info, request)

	require.NoError(t, err)
	body, err := io.ReadAll(reader)
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, common.Unmarshal(body, &payload))
	audio, ok := payload["audio"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "mp3", audio["format"])
	require.NotContains(t, audio, "voice")
}

func TestConvertAudioRequestUsesRequestModelWhenOriginModelIsEmpty(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeAudioSpeech,
	}
	request := dto.AudioRequest{
		Model: "mimo-v2.5-tts",
		Input: "Welcome to MiMo.",
		Voice: "Mia",
	}

	reader, err := adaptor.ConvertAudioRequest(ctx, info, request)

	require.NoError(t, err)
	body, err := io.ReadAll(reader)
	require.NoError(t, err)

	var payload ttsRequest
	require.NoError(t, common.Unmarshal(body, &payload))
	require.Equal(t, "mimo-v2.5-tts", payload.Model)
}

func TestDoResponseForAudioSpeech(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("response_format", "wav")

	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeAudioSpeech,
	}
	info.SetEstimatePromptTokens(7)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"audio":{"data":"UklGRg=="}}}],"usage":{"prompt_tokens":3,"completion_tokens":4,"total_tokens":7}}`)),
	}

	adaptor := &Adaptor{}
	usage, err := adaptor.DoResponse(ctx, resp, info)

	require.Nil(t, err)
	require.Equal(t, "audio/wav", recorder.Header().Get("Content-Type"))
	require.Equal(t, []byte("RIFF"), recorder.Body.Bytes())
	require.Equal(t, &dto.Usage{PromptTokens: 3, CompletionTokens: 4, TotalTokens: 7}, usage)
}

func TestDoResponseForAudioSpeechUsesEstimatedUsage(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("response_format", "mp3")

	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeAudioSpeech,
	}
	info.SetEstimatePromptTokens(11)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"audio":{"data":"UklGRg=="}}}]}`)),
	}

	adaptor := &Adaptor{}
	usage, err := adaptor.DoResponse(ctx, resp, info)

	require.Nil(t, err)
	require.Equal(t, "audio/mpeg", recorder.Header().Get("Content-Type"))
	require.Equal(t, &dto.Usage{PromptTokens: 11, CompletionTokens: 0, TotalTokens: 11}, usage)
}

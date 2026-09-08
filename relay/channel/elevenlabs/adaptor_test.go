package elevenlabs

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertAudioRequestForElevenV3(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	speed := 1.15
	languageCode := "zh"
	request := dto.AudioRequest{
		Model:          "eleven_v3",
		Input:          "[warmly] 你好。",
		Voice:          "voice-id",
		ResponseFormat: "opus",
		Speed:          &speed,
		LanguageCode:   &languageCode,
		VoiceSettings:  []byte(`{"stability":0,"similarity_boost":0.8,"style":0.25,"use_speaker_boost":false,"speed":0.9}`),
	}
	info := &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeAudioSpeech}
	adaptor := &Adaptor{}

	body, err := adaptor.ConvertAudioRequest(ctx, info, request)
	require.NoError(t, err)
	encoded, err := io.ReadAll(body)
	require.NoError(t, err)

	var payload speechRequest
	require.NoError(t, common.Unmarshal(encoded, &payload))
	assert.Equal(t, request.Input, payload.Text)
	assert.Equal(t, request.Model, payload.ModelID)
	require.NotNil(t, payload.LanguageCode)
	assert.Equal(t, languageCode, *payload.LanguageCode)
	require.NotNil(t, payload.VoiceSettings)
	require.NotNil(t, payload.VoiceSettings.Stability)
	assert.Zero(t, *payload.VoiceSettings.Stability)
	require.NotNil(t, payload.VoiceSettings.UseSpeakerBoost)
	assert.False(t, *payload.VoiceSettings.UseSpeakerBoost)
	require.NotNil(t, payload.VoiceSettings.Speed)
	assert.Equal(t, speed, *payload.VoiceSettings.Speed)

	requestURL, err := adaptor.GetRequestURL(&relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeAudioSpeech,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://api.elevenlabs.io/",
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "https://api.elevenlabs.io/v1/text-to-speech/voice-id?output_format=opus_48000_64", requestURL)
}

func TestConvertAudioRequestRejectsInvalidElevenV3Input(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeAudioSpeech}

	tests := []struct {
		name    string
		request dto.AudioRequest
		wantErr string
	}{
		{
			name:    "missing voice ID",
			request: dto.AudioRequest{Model: "eleven_v3", Input: "hello"},
			wantErr: "voice is required",
		},
		{
			name:    "unsupported output format",
			request: dto.AudioRequest{Model: "eleven_v3", Input: "hello", Voice: "voice-id", ResponseFormat: "aac"},
			wantErr: "unsupported ElevenLabs response_format",
		},
		{
			name:    "v3 text exceeds character limit",
			request: dto.AudioRequest{Model: "eleven_v3", Input: strings.Repeat("你", 5001), Voice: "voice-id"},
			wantErr: "at most 5000 characters",
		},
		{
			name:    "voice setting exceeds range",
			request: dto.AudioRequest{Model: "eleven_v3", Input: "hello", Voice: "voice-id", VoiceSettings: []byte(`{"stability":1.1}`)},
			wantErr: "voice_settings.stability must be between 0 and 1",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := (&Adaptor{}).ConvertAudioRequest(ctx, info, test.request)
			require.ErrorContains(t, err, test.wantErr)
		})
	}
}

func TestSetupRequestHeaderUsesElevenLabsAPIKey(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", nil)
	headers := http.Header{}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeAudioSpeech,
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey: "eleven-secret",
		},
	}

	err := (&Adaptor{}).SetupRequestHeader(ctx, &headers, info)

	require.NoError(t, err)
	assert.Equal(t, "eleven-secret", headers.Get("xi-api-key"))
	assert.Equal(t, "application/json", headers.Get("Content-Type"))
	assert.Equal(t, "application/octet-stream", headers.Get("Accept"))
}

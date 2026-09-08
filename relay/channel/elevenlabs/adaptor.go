package elevenlabs

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	channelconstant "github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
)

const maxElevenV3Characters = 5000

type Adaptor struct {
	voiceID      string
	outputFormat string
}

type speechRequest struct {
	Text          string         `json:"text"`
	ModelID       string         `json:"model_id"`
	LanguageCode  *string        `json:"language_code,omitempty"`
	VoiceSettings *voiceSettings `json:"voice_settings,omitempty"`
}

type voiceSettings struct {
	Stability       *float64 `json:"stability,omitempty"`
	SimilarityBoost *float64 `json:"similarity_boost,omitempty"`
	Style           *float64 `json:"style,omitempty"`
	UseSpeakerBoost *bool    `json:"use_speaker_boost,omitempty"`
	Speed           *float64 `json:"speed,omitempty"`
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
	info.IsStream = false
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if info.RelayMode != relayconstant.RelayModeAudioSpeech {
		return "", fmt.Errorf("unsupported relay mode: %d", info.RelayMode)
	}
	if a.voiceID == "" || a.outputFormat == "" {
		return "", errors.New("ElevenLabs audio request is not initialized")
	}

	baseURL := strings.TrimRight(strings.TrimSpace(info.ChannelBaseUrl), "/")
	if baseURL == "" {
		baseURL = channelconstant.GetChannelBaseURL(channelconstant.ChannelTypeElevenLabs)
	}
	if !strings.HasSuffix(baseURL, "/v1") {
		baseURL += "/v1"
	}

	query := url.Values{}
	query.Set("output_format", a.outputFormat)
	return fmt.Sprintf("%s/text-to-speech/%s?%s", baseURL, url.PathEscape(a.voiceID), query.Encode()), nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)
	req.Set("xi-api-key", strings.TrimSpace(info.ApiKey))
	req.Set("Content-Type", "application/json")
	req.Set("Accept", "application/octet-stream")
	return nil
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	if info.RelayMode != relayconstant.RelayModeAudioSpeech {
		return nil, errors.New("unsupported audio relay mode")
	}
	if strings.TrimSpace(request.Input) == "" {
		return nil, errors.New("input is required")
	}
	model := strings.TrimSpace(request.Model)
	if model == "" {
		return nil, errors.New("model is required")
	}
	if model == "eleven_v3" && utf8.RuneCountInString(request.Input) > maxElevenV3Characters {
		return nil, fmt.Errorf("Eleven v3 input must contain at most %d characters", maxElevenV3Characters)
	}

	voiceID := strings.TrimSpace(request.Voice)
	if voiceID == "" {
		return nil, errors.New("voice is required for ElevenLabs; provide a voice ID")
	}
	outputFormat, err := elevenLabsOutputFormat(request.ResponseFormat)
	if err != nil {
		return nil, err
	}

	var settings *voiceSettings
	if len(request.VoiceSettings) > 0 {
		settings = &voiceSettings{}
		if err := common.Unmarshal(request.VoiceSettings, settings); err != nil {
			return nil, fmt.Errorf("invalid ElevenLabs voice_settings: %w", err)
		}
	}
	if request.Speed != nil {
		if settings == nil {
			settings = &voiceSettings{}
		}
		settings.Speed = request.Speed
	}
	if err := validateVoiceSettings(settings); err != nil {
		return nil, err
	}
	if settings != nil &&
		settings.Stability == nil &&
		settings.SimilarityBoost == nil &&
		settings.Style == nil &&
		settings.UseSpeakerBoost == nil &&
		settings.Speed == nil {
		settings = nil
	}

	languageCode := request.LanguageCode
	if languageCode != nil {
		trimmed := strings.TrimSpace(*languageCode)
		if trimmed == "" {
			languageCode = nil
		} else {
			languageCode = &trimmed
		}
	}

	payload := speechRequest{
		Text:          request.Input,
		ModelID:       model,
		LanguageCode:  languageCode,
		VoiceSettings: settings,
	}
	encoded, err := common.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ElevenLabs request: %w", err)
	}

	a.voiceID = voiceID
	a.outputFormat = outputFormat
	return bytes.NewReader(encoded), nil
}

func elevenLabsOutputFormat(responseFormat string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(responseFormat)) {
	case "", "mp3":
		return "mp3_44100_128", nil
	case "opus":
		return "opus_48000_64", nil
	case "pcm":
		return "pcm_24000", nil
	case "wav":
		return "wav_44100", nil
	default:
		return "", fmt.Errorf("unsupported ElevenLabs response_format %q; use mp3, opus, pcm, or wav", responseFormat)
	}
}

func validateVoiceSettings(settings *voiceSettings) error {
	if settings == nil {
		return nil
	}
	for _, setting := range []struct {
		name  string
		value *float64
	}{
		{name: "stability", value: settings.Stability},
		{name: "similarity_boost", value: settings.SimilarityBoost},
		{name: "style", value: settings.Style},
	} {
		if setting.value != nil && (math.IsNaN(*setting.value) || math.IsInf(*setting.value, 0) || *setting.value < 0 || *setting.value > 1) {
			return fmt.Errorf("ElevenLabs voice_settings.%s must be between 0 and 1", setting.name)
		}
	}
	if settings.Speed != nil && (math.IsNaN(*settings.Speed) || math.IsInf(*settings.Speed, 0) || *settings.Speed <= 0) {
		return errors.New("ElevenLabs voice_settings.speed must be greater than 0")
	}
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	return nil, errors.New("ElevenLabs only supports the audio speech endpoint")
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, errors.New("ElevenLabs does not support rerank requests")
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return nil, errors.New("ElevenLabs does not support embedding requests")
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	return nil, errors.New("ElevenLabs does not support image requests")
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	return nil, errors.New("ElevenLabs does not support Responses requests")
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) {
	return nil, errors.New("ElevenLabs does not support Claude requests")
}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	return nil, errors.New("ElevenLabs does not support Gemini requests")
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	if info.RelayMode != relayconstant.RelayModeAudioSpeech {
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("unsupported relay mode: %d", info.RelayMode),
			types.ErrorCodeBadResponse,
			http.StatusBadRequest,
		)
	}
	return openai.OpenaiTTSHandler(c, resp, info), nil
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

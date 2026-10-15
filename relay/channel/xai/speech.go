package xai

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const maxSpeechCharacters = 60000

type speechRequest struct {
	Text         string              `json:"text"`
	VoiceID      string              `json:"voice_id"`
	Language     string              `json:"language"`
	Speed        *float64            `json:"speed,omitempty"`
	OutputFormat *speechOutputFormat `json:"output_format,omitempty"`
}

type speechOutputFormat struct {
	Codec      string `json:"codec"`
	SampleRate *int   `json:"sample_rate,omitempty"`
}

func (a *Adaptor) convertSpeechRequest(request dto.AudioRequest) (io.Reader, error) {
	if !utf8.ValidString(request.Input) || strings.TrimSpace(request.Input) == "" {
		return nil, errors.New("input must contain valid nonempty UTF-8 text")
	}
	characters := utf8.RuneCountInString(request.Input)
	if characters > maxSpeechCharacters {
		return nil, fmt.Errorf("xAI TTS input must contain at most %d characters", maxSpeechCharacters)
	}
	if request.StreamFormat != "" && request.StreamFormat != "audio" {
		return nil, errors.New("xAI TTS only supports binary audio responses; stream_format must be audio")
	}
	if request.Speed != nil && (math.IsNaN(*request.Speed) || math.IsInf(*request.Speed, 0) || *request.Speed < 0.7 || *request.Speed > 1.5) {
		return nil, errors.New("xAI TTS speed must be between 0.7 and 1.5")
	}
	language := "auto"
	if len(request.Language) > 0 {
		if err := common.Unmarshal(request.Language, &language); err != nil || strings.TrimSpace(language) == "" {
			return nil, errors.New("xAI TTS language must be a nonempty string")
		}
		language = strings.TrimSpace(language)
	}
	voice := strings.ToLower(strings.TrimSpace(request.Voice))
	// 游乐场使用 OpenAI 音色名称；原生和自定义音色仍可直接使用。
	switch voice {
	case "", "nova":
		voice = "eve"
	case "alloy":
		voice = "ara"
	case "ash":
		voice = "orion"
	case "ballad":
		voice = "luna"
	case "coral":
		voice = "celeste"
	case "echo":
		voice = "rex"
	case "fable":
		voice = "sal"
	case "onyx":
		voice = "leo"
	case "sage":
		voice = "iris"
	case "shimmer":
		voice = "aurora"
	case "verse":
		voice = "lumen"
	}
	payload := speechRequest{Text: request.Input, VoiceID: voice, Language: language, Speed: request.Speed}
	contentType := "audio/mpeg"
	switch strings.ToLower(strings.TrimSpace(request.ResponseFormat)) {
	case "", "mp3":
	case "wav":
		payload.OutputFormat = &speechOutputFormat{Codec: "wav", SampleRate: common.GetPointer(24000)}
		contentType = "audio/wav"
	case "pcm":
		payload.OutputFormat = &speechOutputFormat{Codec: "pcm", SampleRate: common.GetPointer(24000)}
		contentType = "audio/pcm"
	default:
		return nil, errors.New("xAI TTS response_format must be mp3, wav, or pcm")
	}
	encoded, err := common.Marshal(payload)
	if err != nil {
		return nil, err
	}
	a.speechCharacters, a.speechContentType = characters, contentType
	return bytes.NewReader(encoded), nil
}

func (a *Adaptor) speechResponse(c *gin.Context, response *http.Response) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(response)
	data, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeReadResponseBodyFailed, types.ErrOptionWithSkipRetry())
	}
	if len(data) == 0 || strings.Contains(strings.ToLower(response.Header.Get("Content-Type")), "application/json") {
		return nil, types.NewError(errors.New("xAI TTS did not return audio"), types.ErrorCodeBadResponse, types.ErrOptionWithSkipRetry())
	}
	if response.Header.Get("Content-Type") == "" || response.Header.Get("Content-Type") == "application/octet-stream" {
		response.Header.Set("Content-Type", a.speechContentType)
	}
	service.IOCopyBytesGracefully(c, response, data)
	// xAI 按原始输入字符计费，不从音频时长推算输出 token，也不依赖预估开关。
	common.SetContextKey(c, constant.ContextKeyLocalCountTokens, true)
	usage := &dto.Usage{PromptTokens: a.speechCharacters, TotalTokens: a.speechCharacters}
	usage.PromptTokensDetails.TextTokens = a.speechCharacters
	return usage, nil
}

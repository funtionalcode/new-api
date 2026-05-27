package mimo

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

type ttsRequest struct {
	Model    string       `json:"model"`
	Messages []ttsMessage `json:"messages"`
	Audio    ttsAudio     `json:"audio"`
}

type ttsMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ttsAudio struct {
	Format string `json:"format"`
	Voice  string `json:"voice,omitempty"`
}

type ttsResponse struct {
	Choices []ttsChoice `json:"choices"`
	Usage   dto.Usage   `json:"usage"`
	Error   any         `json:"error"`
}

type ttsChoice struct {
	Message ttsResponseMessage `json:"message"`
}

type ttsResponseMessage struct {
	Audio ttsResponseAudio `json:"audio"`
}

type ttsResponseAudio struct {
	Data string `json:"data"`
}

func getTTSModel(info *relaycommon.RelayInfo, request dto.AudioRequest) string {
	if strings.TrimSpace(info.OriginModelName) != "" {
		return info.OriginModelName
	}
	if info.ChannelMeta != nil && strings.TrimSpace(info.UpstreamModelName) != "" {
		return info.UpstreamModelName
	}
	return request.Model
}

func newTTSRequest(model string, request dto.AudioRequest) ttsRequest {
	messages := make([]ttsMessage, 0, 2)
	if strings.TrimSpace(request.Instructions) != "" {
		messages = append(messages, ttsMessage{Role: "user", Content: request.Instructions})
	}
	messages = append(messages, ttsMessage{Role: "assistant", Content: request.Input})

	return ttsRequest{
		Model:    model,
		Messages: messages,
		Audio: ttsAudio{
			Format: getResponseFormat(request.ResponseFormat),
			Voice:  getVoice(model, request.Voice),
		},
	}
}

func getResponseFormat(responseFormat string) string {
	if strings.TrimSpace(responseFormat) == "" {
		return "mp3"
	}
	return responseFormat
}

func getVoice(model string, voice string) string {
	if model == "mimo-v2.5-tts-voicedesign" {
		return ""
	}
	return voice
}

func handleTTSResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	defer resp.Body.Close()
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, types.NewErrorWithStatusCode(
			errors.New("failed to read mimo response"),
			types.ErrorCodeReadResponseBodyFailed,
			http.StatusInternalServerError,
		)
	}

	var mimoResp ttsResponse
	if unmarshalErr := common.Unmarshal(body, &mimoResp); unmarshalErr != nil {
		return nil, types.NewErrorWithStatusCode(
			errors.New("failed to parse mimo response"),
			types.ErrorCodeBadResponseBody,
			http.StatusInternalServerError,
		)
	}
	if mimoResp.Error != nil {
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("mimo error: %v", mimoResp.Error),
			types.ErrorCodeBadResponse,
			resp.StatusCode,
		)
	}
	if len(mimoResp.Choices) == 0 || mimoResp.Choices[0].Message.Audio.Data == "" {
		return nil, types.NewErrorWithStatusCode(
			errors.New("mimo response missing audio data"),
			types.ErrorCodeBadResponseBody,
			http.StatusInternalServerError,
		)
	}

	audioData, decodeErr := base64.StdEncoding.DecodeString(mimoResp.Choices[0].Message.Audio.Data)
	if decodeErr != nil {
		return nil, types.NewErrorWithStatusCode(
			errors.New("failed to decode mimo audio data"),
			types.ErrorCodeBadResponseBody,
			http.StatusInternalServerError,
		)
	}

	responseFormat, _ := c.Get("response_format")
	contentType := getContentType(responseFormat)
	c.Data(http.StatusOK, contentType, audioData)

	if mimoResp.Usage.TotalTokens > 0 || mimoResp.Usage.PromptTokens > 0 || mimoResp.Usage.CompletionTokens > 0 {
		return &mimoResp.Usage, nil
	}
	return &dto.Usage{
		PromptTokens:     info.GetEstimatePromptTokens(),
		CompletionTokens: 0,
		TotalTokens:      info.GetEstimatePromptTokens(),
	}, nil
}

func getContentType(responseFormat any) string {
	format, ok := responseFormat.(string)
	if !ok {
		format = "mp3"
	}
	contentTypeMap := map[string]string{
		"mp3":  "audio/mpeg",
		"wav":  "audio/wav",
		"flac": "audio/flac",
		"aac":  "audio/aac",
		"pcm":  "audio/pcm",
	}
	if contentType, ok := contentTypeMap[format]; ok {
		return contentType
	}
	return "application/octet-stream"
}

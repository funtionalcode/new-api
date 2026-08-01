package openai

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func OpenaiTTSHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) *dto.Usage {
	// the status code has been judged before, if there is a body reading failure,
	// it should be regarded as a non-recoverable error, so it should not return err for external retry.
	// Analogous to nginx's load balancing, it will only retry if it can't be requested or
	// if the upstream returns a specific status code, once the upstream has already written the header,
	// the subsequent failure of the response body should be regarded as a non-recoverable error,
	// and can be terminated directly.
	defer service.CloseResponseBodyGracefully(resp)
	usage := &dto.Usage{}
	usage.PromptTokens = info.GetEstimatePromptTokens()
	usage.TotalTokens = info.GetEstimatePromptTokens()
	for k, v := range resp.Header {
		if !service.ShouldCopyUpstreamHeader(c, k, v) {
			continue
		}
		c.Writer.Header().Set(k, v[0])
	}
	c.Writer.WriteHeader(resp.StatusCode)

	if info.IsStream {
		helper.StreamScannerHandler(c, resp, info, func(data string, sr *helper.StreamResult) {
			if service.SundaySearch(data, "usage") {
				var simpleResponse dto.SimpleResponse
				if err := common.Unmarshal([]byte(data), &simpleResponse); err != nil {
					logger.LogError(c, err.Error())
					sr.Error(err)
				} else if simpleResponse.Usage.TotalTokens != 0 {
					usage.PromptTokens = simpleResponse.Usage.InputTokens
					usage.CompletionTokens = simpleResponse.OutputTokens
					usage.TotalTokens = simpleResponse.TotalTokens
				}
			}
			if err := helper.StringData(c, data); err != nil {
				sr.Error(err)
			}
		})
	} else {
		common.SetContextKey(c, constant.ContextKeyLocalCountTokens, true)
		// 读取响应体到缓冲区
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			logger.LogError(c, fmt.Sprintf("failed to read TTS response body: %v", err))
			c.Writer.WriteHeaderNow()
			return usage
		}

		// 写入响应到客户端
		c.Writer.WriteHeaderNow()
		_, err = c.Writer.Write(bodyBytes)
		if err != nil {
			logger.LogError(c, fmt.Sprintf("failed to write TTS response: %v", err))
		}

		// 计算音频时长并更新 usage
		audioFormat := "mp3" // 默认格式
		if audioReq, ok := info.Request.(*dto.AudioRequest); ok && audioReq.ResponseFormat != "" {
			audioFormat = audioReq.ResponseFormat
		}

		var duration float64
		var durationErr error

		if audioFormat == "pcm" {
			// PCM 格式没有文件头，根据 OpenAI TTS 的 PCM 参数计算时长
			// 采样率: 24000 Hz, 位深度: 16-bit (2 bytes), 声道数: 1
			const sampleRate = 24000
			const bytesPerSample = 2
			const channels = 1
			duration = float64(len(bodyBytes)) / float64(sampleRate*bytesPerSample*channels)
		} else {
			ext := "." + audioFormat
			reader := bytes.NewReader(bodyBytes)
			duration, durationErr = common.GetAudioDuration(c.Request.Context(), reader, ext)
		}

		usage.PromptTokensDetails.TextTokens = usage.PromptTokens

		if durationErr != nil {
			logger.LogWarn(c, fmt.Sprintf("failed to get audio duration: %v", durationErr))
			// 如果无法获取时长，则设置保底的 CompletionTokens，根据body大小计算
			sizeInKB := float64(len(bodyBytes)) / 1000.0
			estimatedTokens := int(math.Ceil(sizeInKB)) // 粗略估算每KB约等于1 token
			usage.CompletionTokens = estimatedTokens
			usage.CompletionTokenDetails.AudioTokens = estimatedTokens
		} else if duration > 0 {
			// 计算 token: ceil(duration) / 60.0 * 1000，即每分钟 1000 tokens。
			// duration 解析自上游返回的音频元数据，饱和转换防止 int 回绕。
			completionTokens := common.QuotaRound(math.Ceil(duration) / 60.0 * 1000)
			usage.CompletionTokens = completionTokens
			usage.CompletionTokenDetails.AudioTokens = completionTokens
		}
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}

	return usage
}

func OpenaiSTTHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo, responseFormat string) (*types.NewAPIError, *dto.Usage) {
	defer service.CloseResponseBodyGracefully(resp)

	if info.IsStream {
		usage := transcriptionFallbackUsage(info)
		helper.StreamScannerHandler(c, resp, info, func(data string, sr *helper.StreamResult) {
			if parsedUsage := transcriptionUsageFromPayload(common.StringToByteSlice(data)); parsedUsage != nil {
				usage = parsedUsage
			}
			if err := helper.StringData(c, data); err != nil {
				logger.LogError(c, fmt.Sprintf("failed to stream STT response: %v", err))
				sr.Error(err)
			}
		})
		return nil, usage
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError), nil
	}
	// 写入新的 response body
	service.IOCopyBytesGracefully(c, resp, responseBody)

	var responseData transcriptionUsagePayload
	if err := common.Unmarshal(responseBody, &responseData); err == nil {
		if usage := responseData.normalizedUsage(); usage != nil {
			return nil, usage
		}
	}
	if responseData.Duration != nil && *responseData.Duration > 0 {
		return nil, transcriptionUsageFromDuration(*responseData.Duration)
	}

	return nil, transcriptionFallbackUsage(info)
}

type transcriptionUsagePayload struct {
	Duration *float64                 `json:"duration"`
	Usage    *transcriptionUsageAlias `json:"usage"`
}

type transcriptionUsageAlias struct {
	dto.Usage
	InputTokenDetails *dto.InputTokenDetails `json:"input_token_details"`
}

func (p transcriptionUsagePayload) normalizedUsage() *dto.Usage {
	if p.Usage == nil {
		return nil
	}
	usage := &p.Usage.Usage
	if usage.InputTokensDetails == nil && p.Usage.InputTokenDetails != nil {
		usage.InputTokensDetails = p.Usage.InputTokenDetails
	}
	if !hasTranscriptionUsageTokens(usage) {
		return nil
	}
	normalizeTranscriptionUsageDetails(usage)
	return usage
}

func transcriptionUsageFromPayload(data []byte) *dto.Usage {
	var payload transcriptionUsagePayload
	if err := common.Unmarshal(data, &payload); err != nil {
		return nil
	}
	return payload.normalizedUsage()
}

func hasTranscriptionUsageTokens(usage *dto.Usage) bool {
	if usage == nil {
		return false
	}
	return usage.TotalTokens > 0 ||
		usage.PromptTokens > 0 ||
		usage.CompletionTokens > 0 ||
		usage.InputTokens > 0 ||
		usage.OutputTokens > 0
}

func transcriptionUsageFromDuration(duration float64) *dto.Usage {
	tokens := common.QuotaRound(math.Ceil(duration) / 60.0 * 1000)
	usage := &dto.Usage{
		PromptTokens: tokens,
		TotalTokens:  tokens,
	}
	usage.PromptTokensDetails.AudioTokens = tokens
	return usage
}

func transcriptionFallbackUsage(info *relaycommon.RelayInfo) *dto.Usage {
	tokens := 0
	if info != nil {
		tokens = info.GetEstimatePromptTokens()
	}
	usage := &dto.Usage{
		PromptTokens: tokens,
		TotalTokens:  tokens,
	}
	usage.PromptTokensDetails.AudioTokens = tokens
	return usage
}

func normalizeTranscriptionUsageDetails(usage *dto.Usage) {
	if usage == nil {
		return
	}
	if usage.PromptTokens == 0 {
		usage.PromptTokens = usage.InputTokens
	}
	if usage.CompletionTokens == 0 {
		usage.CompletionTokens = usage.OutputTokens
	}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}
	if usage.InputTokensDetails != nil && usage.PromptTokensDetails.AudioTokens == 0 &&
		usage.PromptTokensDetails.TextTokens == 0 && usage.PromptTokensDetails.ImageTokens == 0 {
		usage.PromptTokensDetails = *usage.InputTokensDetails
	}
	if usage.PromptTokens > 0 && usage.PromptTokensDetails.AudioTokens == 0 &&
		usage.PromptTokensDetails.TextTokens == 0 && usage.PromptTokensDetails.ImageTokens == 0 {
		usage.PromptTokensDetails.AudioTokens = usage.PromptTokens
	}
	if usage.CompletionTokens > 0 && usage.CompletionTokenDetails.TextTokens == 0 &&
		usage.CompletionTokenDetails.AudioTokens == 0 && usage.CompletionTokenDetails.ImageTokens == 0 {
		usage.CompletionTokenDetails.TextTokens = usage.CompletionTokens
	}
}

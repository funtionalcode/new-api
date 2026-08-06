package relay

import (
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// HandleClaudeDesktopTokenCountProbe intercepts Claude Desktop's legacy
// max_tokens=1 token-count probe before it can be converted into a billable
// generation request. It supports the original Claude request plus the OpenAI
// Chat Completions and Responses shapes produced by proxies in front of new-api.
func HandleClaudeDesktopTokenCountProbe(c *gin.Context, info *relaycommon.RelayInfo) (bool, *types.NewAPIError) {
	if c == nil || c.Request == nil || info == nil {
		return false, nil
	}

	var tokenMeta *types.TokenCountMeta
	switch request := info.Request.(type) {
	case *dto.ClaudeRequest:
		if c.Request.URL.Path != "/v1/messages" || !isClaudeDesktopClaudeTokenCountProbe(request) {
			return false, nil
		}
		tokenMeta = request.GetTokenCountMeta()
	case *dto.GeneralOpenAIRequest:
		if c.Request.URL.Path != "/v1/chat/completions" || !isClaudeDesktopOpenAITokenCountProbe(request) {
			return false, nil
		}
		tokenMeta = request.GetTokenCountMeta()
	case *dto.OpenAIResponsesRequest:
		if c.Request.URL.Path != "/v1/responses" || !isClaudeDesktopResponsesTokenCountProbe(request) {
			return false, nil
		}
		tokenMeta = request.GetTokenCountMeta()
	default:
		return false, nil
	}

	info.InitChannelMeta(c)
	if err := helper.ModelMappedHelper(c, info, nil); err != nil {
		return true, types.NewError(err, types.ErrorCodeChannelModelMappedError, types.ErrOptionWithSkipRetry())
	}

	inputTokens, err := service.CountRequestToken(c, tokenMeta, info, info.UpstreamModelName)
	if err != nil {
		return true, types.NewError(err, types.ErrorCodeCountTokenFailed, types.ErrOptionWithSkipRetry())
	}
	info.SetEstimatePromptTokens(inputTokens)

	responseID := info.RequestId
	if responseID == "" {
		responseID = common.NewRequestId()
	}

	switch info.Request.(type) {
	case *dto.ClaudeRequest:
		c.JSON(http.StatusOK, gin.H{
			"id":            "msg_" + responseID,
			"type":          "message",
			"role":          "assistant",
			"model":         info.OriginModelName,
			"content":       []gin.H{{"type": "text", "text": ""}},
			"stop_reason":   "end_turn",
			"stop_sequence": nil,
			"usage": gin.H{
				"input_tokens":  inputTokens,
				"output_tokens": 0,
			},
		})
	case *dto.GeneralOpenAIRequest:
		c.JSON(http.StatusOK, gin.H{
			"id":      "chatcmpl-" + responseID,
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   info.OriginModelName,
			"choices": []gin.H{{
				"index": 0,
				"message": gin.H{
					"role":    "assistant",
					"content": "",
				},
				"finish_reason": "stop",
			}},
			"usage": gin.H{
				"prompt_tokens":     inputTokens,
				"completion_tokens": 0,
				"total_tokens":      inputTokens,
			},
		})
	case *dto.OpenAIResponsesRequest:
		c.JSON(http.StatusOK, gin.H{
			"id":                "resp_" + responseID,
			"object":            "response",
			"created_at":        time.Now().Unix(),
			"status":            "completed",
			"model":             info.OriginModelName,
			"max_output_tokens": 1,
			"output": []gin.H{{
				"id":     "msg_" + responseID,
				"type":   "message",
				"status": "completed",
				"role":   "assistant",
				"content": []gin.H{{
					"type":        "output_text",
					"text":        "",
					"annotations": []any{},
				}},
			}},
			"usage": gin.H{
				"input_tokens":  inputTokens,
				"output_tokens": 0,
				"total_tokens":  inputTokens,
			},
		})
	}

	return true, nil
}

func isClaudeDesktopClaudeTokenCountProbe(request *dto.ClaudeRequest) bool {
	if request == nil || request.MaxTokens == nil || *request.MaxTokens != 1 || len(request.Messages) != 1 || len(request.GetTools()) == 0 {
		return false
	}
	if request.Stream != nil && *request.Stream {
		return false
	}
	message := request.Messages[0]
	return message.Role == "user" && message.IsStringContent() && strings.TrimSpace(message.GetStringContent()) == "count"
}

func isClaudeDesktopOpenAITokenCountProbe(request *dto.GeneralOpenAIRequest) bool {
	if request == nil || request.GetMaxTokens() != 1 || len(request.Messages) != 1 || len(request.Tools) == 0 {
		return false
	}
	if request.Stream != nil && *request.Stream {
		return false
	}
	message := request.Messages[0]
	return message.Role == "user" && message.IsStringContent() && strings.TrimSpace(message.StringContent()) == "count"
}

func isClaudeDesktopResponsesTokenCountProbe(request *dto.OpenAIResponsesRequest) bool {
	if request == nil || request.MaxOutputTokens == nil || *request.MaxOutputTokens != 1 {
		return false
	}
	if request.Stream != nil && *request.Stream {
		return false
	}

	var tools []any
	if len(request.Tools) == 0 || common.Unmarshal(request.Tools, &tools) != nil || len(tools) == 0 {
		return false
	}

	switch common.GetJsonType(request.Input) {
	case "string":
		var input string
		return common.Unmarshal(request.Input, &input) == nil && strings.TrimSpace(input) == "count"
	case "array":
		var items []map[string]any
		if common.Unmarshal(request.Input, &items) != nil || len(items) != 1 {
			return false
		}
		item := items[0]
		if role, _ := item["role"].(string); role != "" && role != "user" {
			return false
		}
		if itemType, _ := item["type"].(string); itemType != "" && itemType != "message" {
			return false
		}
		switch content := item["content"].(type) {
		case string:
			return strings.TrimSpace(content) == "count"
		case []any:
			if len(content) != 1 {
				return false
			}
			part, ok := content[0].(map[string]any)
			if !ok || part["type"] != "input_text" {
				return false
			}
			text, _ := part["text"].(string)
			return strings.TrimSpace(text) == "count"
		}
	}
	return false
}

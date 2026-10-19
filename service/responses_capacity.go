package service

import (
	"bytes"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
)

// ResponsesCapacityGuard 仅暂存有界的响应元信息，业务内容或工具事件立即提交。
// Started 为真后只隐藏容量错误，不允许重复生成已提交的响应。
type ResponsesCapacityGuard struct {
	Started bool
	pending [][]byte
	size    int
}

func (g *ResponsesCapacityGuard) Flush() [][]byte {
	frames := g.pending
	g.pending, g.size = nil, 0
	if len(frames) > 0 {
		g.Started = true
	}
	return frames
}

func (g *ResponsesCapacityGuard) Observe(event *dto.ResponsesStreamResponse, raw []byte) ([][]byte, *types.NewAPIError) {
	var failure *types.NewAPIError
	if event.Type == "error" || event.Type == "response.error" || event.Type == "response.failed" {
		var envelope struct {
			Error any `json:"error"`
		}
		_ = common.Unmarshal(raw, &envelope)
		upstream := dto.GetOpenAIError(envelope.Error)
		if upstream == nil && event.Response != nil {
			upstream = event.Response.GetOpenAIError()
		}
		if upstream == nil && event.Message != "" {
			upstream = &types.OpenAIError{Message: event.Message, Code: event.Code}
		}
		if upstream != nil {
			candidate := types.WithOpenAIError(*upstream, http.StatusInternalServerError)
			if candidate.GetErrorCode() == types.ErrorCodeUpstreamModelCapacity {
				failure = candidate
			}
		}
	}
	produced := false
	if response := event.Response; response != nil {
		produced = len(response.Output) > 0
		if u := response.Usage; u != nil {
			produced = produced || u.InputTokens != 0 || u.OutputTokens != 0 || u.TotalTokens != 0 || u.PromptTokens != 0 || u.CompletionTokens != 0
		}
	}
	if failure != nil {
		if !g.Started && !produced {
			g.pending, g.size = nil, 0
			return nil, failure
		}
		var frame map[string]any
		if err := common.Unmarshal(raw, &frame); err != nil {
			return nil, failure
		}
		if _, exists := frame["error"]; exists || event.Type == "error" {
			frame["error"] = failure.ToOpenAIError()
		}
		if response, ok := frame["response"].(map[string]any); ok {
			response["error"] = failure.ToOpenAIError()
		}
		if _, exists := frame["message"]; exists {
			frame["message"] = "Internal server error"
		}
		if _, exists := frame["code"]; exists {
			frame["code"] = "internal_error"
		}
		if _, exists := frame["status"]; exists && event.Type == "error" {
			frame["status"] = http.StatusInternalServerError
		}
		var err error
		raw, err = common.Marshal(frame)
		if err != nil {
			return nil, failure
		}
	}
	if failure == nil && !g.Started && !produced && (event.Type == "response.created" || event.Type == "response.in_progress") && len(g.pending) < 2 && g.size+len(raw) <= 64<<10 {
		g.pending = append(g.pending, bytes.Clone(raw))
		g.size += len(raw)
		return nil, nil
	}
	frames := g.Flush()
	g.Started = true
	return append(frames, raw), failure
}

package service

import (
	"bytes"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
)

// ResponsesCapacityGuard 仅暂存有界的响应元信息，业务内容或工具事件立即提交。
// HTTP 开始写入后只隐藏错误；WebSocket 可在仅发布元信息时保留同连接重试能力。
// Committed 为真说明已有业务输出或控制副作用，禁止重放。
type ResponsesCapacityGuard struct {
	Started             bool
	Committed           bool
	AllowMetadataReplay bool
	pending             [][]byte
	size                int
}

func (g *ResponsesCapacityGuard) CanRetry() bool {
	return !g.Committed && (!g.Started || g.AllowMetadataReplay)
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
		candidates := []*types.OpenAIError{dto.GetOpenAIError(envelope.Error)}
		if event.Response != nil {
			candidates = append(candidates, event.Response.GetOpenAIError())
		}
		candidates = append(candidates, &types.OpenAIError{Message: event.Message, Code: event.Code})
		for _, upstream := range candidates {
			if upstream == nil {
				continue
			}
			candidate := types.WithOpenAIError(*upstream, http.StatusInternalServerError)
			if candidate.GetErrorCode() == types.ErrorCodeUpstreamModelCapacity {
				failure = candidate
				break
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
		if g.CanRetry() && !produced {
			g.pending, g.size = nil, 0
			return nil, failure
		}
		var frame map[string]any
		if err := common.Unmarshal(raw, &frame); err != nil {
			return nil, failure
		}
		if _, exists := frame["error"]; exists || event.Type == "error" {
			masked := failure.ToOpenAIError()
			publicError := map[string]any{"type": masked.Type, "code": masked.Code, "message": masked.Message}
			if original, ok := frame["error"].(map[string]any); ok {
				for _, key := range []string{"response_id", "event_id", "stream_id"} {
					if id, ok := original[key].(string); ok {
						publicError[key] = id
					}
				}
			}
			frame["error"] = publicError
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
	if produced || (event.Type != "response.created" && event.Type != "response.in_progress") {
		g.Committed = true
	}
	return append(frames, raw), failure
}

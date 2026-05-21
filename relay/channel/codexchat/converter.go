package codexchat

import (
	"bufio"
	"io"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/google/uuid"
)

// ChatGPTConversationRequest ChatGPT conversation API 请求格式
type ChatGPTConversationRequest struct {
	Action                     string            `json:"action"`
	Messages                   []chatGPTMessage   `json:"messages"`
	ParentMessageID            string            `json:"parent_message_id"`
	Model                      string            `json:"model"`
	HistoryAndTrainingDisabled bool              `json:"history_and_training_disabled"`
	TimezoneOffsetMin          int               `json:"timezone_offset_min"`
	Timezone                   string            `json:"timezone"`
	Suggestions                []string          `json:"suggestions"`
	ConversationMode           conversationMode  `json:"conversation_mode"`
	SupportsBuffering          bool              `json:"supports_buffering"`
}

type chatGPTMessage struct {
	ID       string                 `json:"id"`
	Author   messageAuthor          `json:"author"`
	Content  messageContent         `json:"content"`
	Metadata map[string]any         `json:"metadata"`
}

type messageAuthor struct {
	Role string `json:"role"`
}

type messageContent struct {
	ContentType string   `json:"content_type"`
	Parts       []string `json:"parts"`
}

type conversationMode struct {
	Kind string `json:"kind"`
}

// chatGPTSSEEvent ChatGPT SSE 响应事件
type chatGPTSSEEvent struct {
	Type           string              `json:"type"`
	Message        *chatGPTRespMessage `json:"message,omitempty"`
	ConversationID string              `json:"conversation_id,omitempty"`
}

type chatGPTRespMessage struct {
	ID      string         `json:"id"`
	Author  messageAuthor  `json:"author"`
	Content messageContent `json:"content"`
	Status  string         `json:"status"`
}

// convertToChatGPTConversation 将 OpenAI 请求转换为 ChatGPT conversation 格式
func convertToChatGPTConversation(req *dto.GeneralOpenAIRequest) *ChatGPTConversationRequest {
	messages := make([]chatGPTMessage, 0, len(req.Messages))
	for _, msg := range req.Messages {
		parts := extractTextParts(msg)
		messages = append(messages, chatGPTMessage{
			ID: uuid.New().String(),
			Author: messageAuthor{
				Role: msg.Role,
			},
			Content: messageContent{
				ContentType: "text",
				Parts:       parts,
			},
			Metadata: map[string]any{},
		})
	}

	return &ChatGPTConversationRequest{
		Action:                     "next",
		Messages:                   messages,
		ParentMessageID:            uuid.New().String(),
		Model:                      req.Model,
		HistoryAndTrainingDisabled: false,
		TimezoneOffsetMin:          -480,
		Timezone:                   "Asia/Shanghai",
		Suggestions:                []string{},
		ConversationMode:           conversationMode{Kind: "primary_assistant"},
		SupportsBuffering:          true,
	}
}

// extractTextParts 从 Message.Content 中提取文本
func extractTextParts(msg dto.Message) []string {
	if msg.IsStringContent() {
		return []string{msg.StringContent()}
	}
	contents := msg.ParseContent()
	var parts []string
	for _, c := range contents {
		if c.Type == dto.ContentTypeText && c.Text != "" {
			parts = append(parts, c.Text)
		}
	}
	if len(parts) == 0 {
		parts = []string{""}
	}
	return parts
}

// parseChatGPTSSEStream 解析 ChatGPT SSE 流，返回事件通道
func parseChatGPTSSEStream(reader io.Reader) <-chan chatGPTSSEEvent {
	ch := make(chan chatGPTSSEEvent, 64)
	go func() {
		defer close(ch)
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				return
			}
			var event chatGPTSSEEvent
			if err := common.Unmarshal([]byte(data), &event); err != nil {
				continue
			}
			ch <- event
		}
	}()
	return ch
}

// convertToOpenAIStreamChunk 将 ChatGPT 事件转换为 OpenAI 流式 chunk
func convertToOpenAIStreamChunk(event chatGPTSSEEvent, model string, respID string) *dto.ChatCompletionsStreamResponse {
	if event.Message == nil {
		return nil
	}
	msg := event.Message
	if msg.Content.ContentType != "text" || len(msg.Content.Parts) == 0 {
		return nil
	}
	content := strings.Join(msg.Content.Parts, "")
	if content == "" {
		return nil
	}

	return &dto.ChatCompletionsStreamResponse{
		Id:      respID,
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index: 0,
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					Content: &content,
				},
			},
		},
	}
}

// buildStopChunk 构建结束 chunk
func buildStopChunk(model string, respID string) *dto.ChatCompletionsStreamResponse {
	finishReason := "stop"
	return &dto.ChatCompletionsStreamResponse{
		Id:      respID,
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index:        0,
				FinishReason: &finishReason,
				Delta:        dto.ChatCompletionsStreamResponseChoiceDelta{},
			},
		},
	}
}

// buildUsageChunk 构建 usage chunk
func buildUsageChunk(model string, respID string, promptTokens int, completionTokens int) *dto.ChatCompletionsStreamResponse {
	stopReason := "stop"
	return &dto.ChatCompletionsStreamResponse{
		Id:      respID,
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index:        0,
				FinishReason: &stopReason,
				Delta:        dto.ChatCompletionsStreamResponseChoiceDelta{},
			},
		},
		Usage: &dto.Usage{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
		},
	}
}

// MarshalJSON 用于安全地序列化为 JSON
func MarshalJSON(v any) ([]byte, error) {
	return common.Marshal(v)
}

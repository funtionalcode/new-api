package cursor

import (
	"bytes"
	"context"
	"crypto/hmac"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Adaptor struct{}

func (a *Adaptor) Init(_ *relaycommon.RelayInfo) {}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if info == nil {
		return "", errors.New("cursor channel: relay info is required")
	}
	if info.RelayMode != 0 && info.RelayMode != relayconstant.RelayModeChatCompletions {
		return "", errors.New("cursor channel: only /v1/chat/completions is supported")
	}
	return relaycommon.GetFullRequestURL(strings.TrimRight(info.ChannelBaseUrl, "/"), "/v1/agents", info.ChannelType), nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, header *http.Header, info *relaycommon.RelayInfo) error {
	if info == nil {
		return errors.New("cursor channel: relay info is required")
	}
	key := strings.TrimSpace(info.ApiKey)
	if key == "" {
		return errors.New("cursor channel: API key is required")
	}

	channel.SetupApiRequestHeader(info, c, header)
	header.Set("Authorization", "Bearer "+key)
	header.Set("Content-Type", "application/json")
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("cursor channel: request is required")
	}
	if strings.TrimSpace(request.Model) == "" {
		return nil, errors.New("cursor channel: model is required")
	}
	if len(request.Messages) == 0 {
		return nil, errors.New("cursor channel: messages are required")
	}
	if len(request.Tools) > 0 || len(request.Functions) > 0 || request.ToolChoice != nil || len(request.FunctionCall) > 0 {
		return nil, errors.New("cursor channel: tools are not supported in text chat mode")
	}

	metadata := cursorMetadata{}
	if len(request.Metadata) > 0 {
		var rawMetadata map[string]any
		if err := common.Unmarshal(request.Metadata, &rawMetadata); err != nil {
			return nil, fmt.Errorf("cursor channel: invalid metadata: %w", err)
		}
		if value, exists := rawMetadata[cursorAgentIDMetadataKey]; exists {
			agentID, ok := value.(string)
			if !ok {
				return nil, errors.New("cursor channel: metadata.cursor_agent_id must be a string")
			}
			metadata.AgentID = agentID
		}
		if value, exists := rawMetadata[cursorPersistentMetadataKey]; exists {
			persistent, ok := value.(bool)
			if !ok {
				return nil, errors.New("cursor channel: metadata.cursor_persistent must be a boolean")
			}
			metadata.Persistent = persistent
		}
	}
	if c != nil && c.Request != nil {
		if headerAgentID := strings.TrimSpace(c.Request.Header.Get(cursorAgentIDHeader)); headerAgentID != "" {
			metadata.AgentID = headerAgentID
		}
		if persistent, err := strconv.ParseBool(strings.TrimSpace(c.Request.Header.Get(cursorPersistentHeader))); err == nil {
			metadata.Persistent = persistent
		}
	}
	metadata.AgentID = strings.TrimSpace(metadata.AgentID)
	if metadata.AgentID != "" {
		if err := validateCursorAgentID(c, metadata.AgentID); err != nil {
			return nil, err
		}
		metadata.Persistent = true
	}

	transcriptMessages := make([]cursorTranscriptMessage, 0, len(request.Messages))
	for index := range request.Messages {
		message := &request.Messages[index]
		switch message.Role {
		case "system", "developer", "user", "assistant":
		default:
			return nil, fmt.Errorf("cursor channel: message role %q is not supported in text chat mode", message.Role)
		}
		if len(message.ToolCalls) > 0 || message.ToolCallId != "" {
			return nil, errors.New("cursor channel: tool messages are not supported in text chat mode")
		}
		content, err := textContentForCursor(message)
		if err != nil {
			return nil, err
		}
		transcriptMessages = append(transcriptMessages, cursorTranscriptMessage{Role: message.Role, Content: content})
	}

	if c != nil {
		c.Set(cursorAgentIDContextKey, metadata.AgentID)
		c.Set(cursorPersistentContextKey, metadata.Persistent)
	}

	if metadata.AgentID != "" {
		latest := transcriptMessages[len(transcriptMessages)-1]
		if latest.Role != "user" {
			return nil, errors.New("cursor channel: a persistent agent continuation must end with a user message")
		}
		return &createRunRequest{Prompt: cursorPrompt{Text: latest.Content}}, nil
	}
	if info != nil && info.IsChannelTest {
		return &createAgentRequest{
			Prompt: cursorPrompt{Text: "Reply exactly with OK. Do not use tools or inspect any repository."},
			Model:  cursorModelSelection{ID: request.Model},
			Name:   "new-api channel test",
		}, nil
	}

	var transcript strings.Builder
	transcript.WriteString("You are continuing a text-only chat. Do not use tools, edit files, run commands, or create artifacts.\n")
	transcript.WriteString("Conversation transcript follows as JSON Lines. Respect each message role and all prior context.\n")
	for _, message := range transcriptMessages {
		line, err := common.Marshal(message)
		if err != nil {
			return nil, fmt.Errorf("cursor channel: encode conversation: %w", err)
		}
		transcript.Write(line)
		transcript.WriteByte('\n')
	}
	transcript.WriteString("Return only the assistant reply to the final user message.")

	return &createAgentRequest{
		Prompt: cursorPrompt{Text: transcript.String()},
		Model:  cursorModelSelection{ID: request.Model},
		Name:   "new-api text chat",
	}, nil
}

func textContentForCursor(message *dto.Message) (string, error) {
	if message == nil || message.Content == nil {
		return "", errors.New("cursor channel: text-only message content is required")
	}
	if content, ok := message.Content.(string); ok {
		return content, nil
	}

	var content strings.Builder
	appendPart := func(part dto.MediaContent) error {
		if part.Type != dto.ContentTypeText {
			return errors.New("cursor channel: text-only message content is required")
		}
		content.WriteString(part.Text)
		return nil
	}

	switch parts := message.Content.(type) {
	case []dto.MediaContent:
		for _, part := range parts {
			if err := appendPart(part); err != nil {
				return "", err
			}
		}
	case []any:
		for _, rawPart := range parts {
			if part, ok := rawPart.(dto.MediaContent); ok {
				if err := appendPart(part); err != nil {
					return "", err
				}
				continue
			}
			part, ok := rawPart.(map[string]any)
			if !ok || part["type"] != dto.ContentTypeText {
				return "", errors.New("cursor channel: text-only message content is required")
			}
			text, ok := part["text"].(string)
			if !ok {
				return "", errors.New("cursor channel: text-only message content is required")
			}
			content.WriteString(text)
		}
	default:
		return "", errors.New("cursor channel: text-only message content is required")
	}
	return content.String(), nil
}

func validateCursorAgentID(c *gin.Context, agentID string) error {
	if !strings.HasPrefix(agentID, "bc-") {
		return errors.New("cursor channel: cursor_agent_id must start with bc-")
	}
	if _, err := uuid.Parse(strings.TrimPrefix(agentID, "bc-")); err != nil {
		return errors.New("cursor channel: cursor_agent_id is invalid")
	}
	if c == nil || c.Request == nil {
		return errors.New("cursor channel: request context is required for a persistent agent")
	}
	signature := strings.TrimSpace(c.Request.Header.Get(cursorAgentSignatureHeader))
	if signature == "" {
		return errors.New("cursor channel: persistent agent signature is missing")
	}
	requestUserID := c.GetInt("id")
	if requestUserID <= 0 {
		return errors.New("cursor channel: persistent agent user identity is missing")
	}
	parts := strings.SplitN(signature, ".", 2)
	if len(parts) != 2 || parts[0] != cursorAgentSignatureVersion || parts[1] == "" {
		return errors.New("cursor channel: persistent agent signature version is invalid")
	}
	expected := cursorAgentSignature(requestUserID, agentID)
	if !hmac.Equal([]byte(signature), []byte(expected)) {
		return errors.New("cursor channel: persistent agent does not belong to this user")
	}
	return nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	if requestBody == nil {
		return nil, errors.New("cursor channel: request body is required")
	}
	body, err := io.ReadAll(requestBody)
	if err != nil {
		return nil, fmt.Errorf("cursor channel: read request body: %w", err)
	}

	agentID := c.GetString(cursorAgentIDContextKey)
	persistent := c.GetBool(cursorPersistentContextKey)
	if persistent && agentID != "" {
		setCursorAgentResponseHeaders(c, agentID)
	}
	requestPath := "/v1/agents"
	if agentID != "" {
		requestPath = "/v1/agents/" + url.PathEscape(agentID) + "/runs"
	}

	response, err := a.doCursorAPIRequest(c.Request.Context(), c, info, http.MethodPost, requestPath, bytes.NewReader(body), false)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return response, nil
	}
	responseBody, err := io.ReadAll(response.Body)
	service.CloseResponseBodyGracefully(response)
	if err != nil {
		return nil, fmt.Errorf("cursor channel: read create response: %w", err)
	}

	runID := ""
	if agentID == "" {
		var created createAgentResponse
		if err := common.Unmarshal(responseBody, &created); err != nil {
			return nil, fmt.Errorf("cursor channel: decode create agent response: %w", err)
		}
		agentID = created.Agent.ID
		runID = created.Run.ID
		if runID == "" {
			runID = created.LatestRunID
		}
		if runID == "" {
			runID = created.Agent.LatestRunID
		}
	} else {
		var created createRunResponse
		if err := common.Unmarshal(responseBody, &created); err != nil {
			return nil, fmt.Errorf("cursor channel: decode create run response: %w", err)
		}
		runID = created.Run.ID
	}
	if agentID == "" || runID == "" {
		if agentID != "" && !persistent {
			a.finishCursorRun(c, info, agentID, runID, false, true)
		}
		return nil, errors.New("cursor channel: create response is missing agent or run ID")
	}
	if persistent {
		setCursorAgentResponseHeaders(c, agentID)
	}

	streamPath := "/v1/agents/" + url.PathEscape(agentID) + "/runs/" + url.PathEscape(runID) + "/stream"
	streamResponse, err := a.doCursorAPIRequest(c.Request.Context(), c, info, http.MethodGet, streamPath, nil, true)
	if err != nil {
		a.finishCursorRun(c, info, agentID, runID, persistent, false)
		return nil, err
	}
	streamResponse.Header.Set(cursorAgentIDInternalHeader, agentID)
	streamResponse.Header.Set(cursorRunIDInternalHeader, runID)
	streamResponse.Header.Set(cursorPersistentInternalKey, strconv.FormatBool(persistent))
	streamResponse.Header.Set(cursorClientStreamHeader, strconv.FormatBool(info.IsStream))
	if info.IsChannelTest {
		streamResponse.Header.Set(cursorSkipRemoteUsageHeader, "true")
	}
	if !info.IsStream {
		streamResponse.Header.Set("Content-Type", cursorEventStreamContentType)
	}
	if streamResponse.StatusCode != http.StatusOK {
		a.finishCursorRun(c, info, agentID, runID, persistent, false)
	}
	return streamResponse, nil
}

func (a *Adaptor) doCursorAPIRequest(ctx context.Context, c *gin.Context, info *relaycommon.RelayInfo, method string, path string, body io.Reader, stream bool) (*http.Response, error) {
	requestURL := relaycommon.GetFullRequestURL(strings.TrimRight(info.ChannelBaseUrl, "/"), path, info.ChannelType)
	if body == nil {
		body = http.NoBody
	}
	req, err := http.NewRequestWithContext(ctx, method, requestURL, body)
	if err != nil {
		return nil, fmt.Errorf("cursor channel: create upstream request: %w", err)
	}
	headers := req.Header
	if err := a.SetupRequestHeader(c, &headers, info); err != nil {
		return nil, err
	}
	if stream {
		req.Header.Set("Accept", "text/event-stream")
	}
	overrides, err := channel.ResolveHeaderOverride(info, c)
	if err != nil {
		return nil, err
	}
	for name, value := range overrides {
		req.Header.Set(name, value)
		if strings.EqualFold(name, "Host") {
			req.Host = value
		}
	}

	requestInfo := *info
	requestInfo.IsStream = stream && info.IsStream
	response, err := channel.DoRequest(c, req, &requestInfo)
	if err != nil {
		return nil, fmt.Errorf("cursor channel: upstream request failed: %w", err)
	}
	return response, nil
}

func (a *Adaptor) GetModelList() []string { return ModelList }

func (a *Adaptor) GetChannelName() string { return ChannelName }

func (a *Adaptor) ConvertRerankRequest(*gin.Context, int, dto.RerankRequest) (any, error) {
	return nil, errors.New("cursor channel: /v1/rerank endpoint not supported")
}

func (a *Adaptor) ConvertEmbeddingRequest(*gin.Context, *relaycommon.RelayInfo, dto.EmbeddingRequest) (any, error) {
	return nil, errors.New("cursor channel: /v1/embeddings endpoint not supported")
}

func (a *Adaptor) ConvertAudioRequest(*gin.Context, *relaycommon.RelayInfo, dto.AudioRequest) (io.Reader, error) {
	return nil, errors.New("cursor channel: audio endpoints are not supported")
}

func (a *Adaptor) ConvertImageRequest(*gin.Context, *relaycommon.RelayInfo, dto.ImageRequest) (any, error) {
	return nil, errors.New("cursor channel: image endpoints are not supported")
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(*gin.Context, *relaycommon.RelayInfo, dto.OpenAIResponsesRequest) (any, error) {
	return nil, errors.New("cursor channel: /v1/responses endpoint not supported")
}

func (a *Adaptor) ConvertClaudeRequest(*gin.Context, *relaycommon.RelayInfo, *dto.ClaudeRequest) (any, error) {
	return nil, errors.New("cursor channel: /v1/messages endpoint not supported")
}

func (a *Adaptor) ConvertGeminiRequest(*gin.Context, *relaycommon.RelayInfo, *dto.GeminiChatRequest) (any, error) {
	return nil, errors.New("cursor channel: Gemini endpoint not supported")
}

var _ channel.Adaptor = (*Adaptor)(nil)

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
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
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
		if headerAgentID := strings.TrimSpace(c.Request.Header.Get(constant.CursorAgentIDHeader)); headerAgentID != "" {
			metadata.AgentID = headerAgentID
		}
		if persistent, err := strconv.ParseBool(strings.TrimSpace(c.Request.Header.Get(constant.CursorPersistentHeader))); err == nil {
			metadata.Persistent = persistent
		}
	}
	metadata.AgentID = strings.TrimSpace(metadata.AgentID)
	if metadata.AgentID != "" {
		metadata.Persistent = true
		if err := validateCursorAgentChannel(c, info); err != nil {
			return nil, err
		}
		if err := validateCursorAgentID(c, info, metadata.AgentID); err != nil {
			if !errors.Is(err, errCursorLegacyAgentSignature) && !errors.Is(err, errCursorAgentSignatureMismatch) {
				return nil, err
			}
			metadata.AgentID = ""
		}
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
		if strings.TrimSpace(latest.Content) == cursorCleanupCommand {
			common.SetContextKey(c, constant.ContextKeyCursorAgentLifecycle, constant.CursorAgentLifecycleDelete)
			return &deleteAgentRequest{}, nil
		}
		return &createRunRequest{Prompt: cursorPrompt{Text: latest.Content}}, nil
	}
	latest := transcriptMessages[len(transcriptMessages)-1]
	if latest.Role == "user" && strings.TrimSpace(latest.Content) == cursorCleanupCommand {
		return nil, errors.New("cursor channel: no active Cursor Agent can be deleted")
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

func validateCursorAgentID(c *gin.Context, info *relaycommon.RelayInfo, agentID string) error {
	if !strings.HasPrefix(agentID, "bc-") {
		return errors.New("cursor channel: cursor_agent_id must start with bc-")
	}
	if _, err := uuid.Parse(strings.TrimPrefix(agentID, "bc-")); err != nil {
		return errors.New("cursor channel: cursor_agent_id is invalid")
	}
	if c == nil || c.Request == nil {
		return errors.New("cursor channel: request context is required for a persistent agent")
	}
	signature := strings.TrimSpace(c.Request.Header.Get(constant.CursorAgentSignatureHeader))
	if signature == "" {
		return errors.New("cursor channel: persistent agent signature is missing")
	}
	requestUserID := c.GetInt("id")
	if requestUserID <= 0 {
		return errors.New("cursor channel: persistent agent user identity is missing")
	}
	if info == nil || strings.TrimSpace(info.ApiKey) == "" {
		return errors.New("cursor channel: persistent agent channel key is missing")
	}
	parts := strings.SplitN(signature, ".", 2)
	if len(parts) == 2 && parts[0] == "v1" && parts[1] != "" {
		return errCursorLegacyAgentSignature
	}
	if len(parts) != 2 || parts[0] != cursorAgentSignatureVersion || parts[1] == "" {
		return errors.New("cursor channel: persistent agent signature version is invalid")
	}
	expected := cursorAgentSignature(requestUserID, agentID, info.ChannelId, info.ChannelMultiKeyIndex, info.ApiKey)
	if !hmac.Equal([]byte(signature), []byte(expected)) {
		return errCursorAgentSignatureMismatch
	}
	return nil
}

func validateCursorAgentChannel(c *gin.Context, info *relaycommon.RelayInfo) error {
	if c == nil || c.Request == nil || info == nil {
		return errors.New("cursor channel: request channel context is required for a persistent agent")
	}
	channelID, err := strconv.Atoi(strings.TrimSpace(c.Request.Header.Get(constant.CursorAgentChannelIDHeader)))
	if err != nil || channelID <= 0 {
		return errors.New("cursor channel: persistent agent channel is missing or invalid")
	}
	if channelID != info.ChannelId {
		return errors.New("cursor channel: persistent agent belongs to a different channel")
	}
	keyIndex, err := strconv.Atoi(strings.TrimSpace(c.Request.Header.Get(constant.CursorAgentKeyIndexHeader)))
	if err != nil || keyIndex < 0 {
		return errors.New("cursor channel: persistent agent key index is missing or invalid")
	}
	if keyIndex != info.ChannelMultiKeyIndex {
		return errors.New("cursor channel: persistent agent belongs to a different channel key")
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
	if common.GetContextKeyString(c, constant.ContextKeyCursorAgentLifecycle) == constant.CursorAgentLifecycleDelete {
		agentID := c.GetString(cursorAgentIDContextKey)
		if err := a.DeletePersistentAgent(c, info, agentID); err != nil {
			return nil, err
		}
		c.Header(constant.CursorAgentDeletedHeader, "true")
		responseHeader := make(http.Header)
		responseHeader.Set(cursorAgentLifecycleHeader, constant.CursorAgentLifecycleDelete)
		responseHeader.Set(cursorClientStreamHeader, strconv.FormatBool(info.IsStream))
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     responseHeader,
			Body:       io.NopCloser(strings.NewReader("")),
		}, nil
	}

	agentID := c.GetString(cursorAgentIDContextKey)
	persistent := c.GetBool(cursorPersistentContextKey)
	creatingPersistentAgent := persistent && agentID == ""
	if persistent && agentID != "" {
		setCursorAgentResponseHeaders(c, info, agentID)
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
		setCursorAgentResponseHeaders(c, info, agentID)
	}

	streamPath := "/v1/agents/" + url.PathEscape(agentID) + "/runs/" + url.PathEscape(runID) + "/stream"
	streamResponse, err := a.doCursorAPIRequest(c.Request.Context(), c, info, http.MethodGet, streamPath, nil, true)
	if err != nil {
		a.finishCursorRun(c, info, agentID, runID, persistent, false)
		return nil, err
	}
	if streamResponse.StatusCode != http.StatusOK {
		responseBody, readErr := io.ReadAll(streamResponse.Body)
		service.CloseResponseBodyGracefully(streamResponse)
		if readErr != nil {
			a.finishCursorRun(c, info, agentID, runID, persistent, false)
			return nil, fmt.Errorf("cursor channel: read stream error response: %w", readErr)
		}
		var errorResponse cursorAPIErrorResponse
		unmarshalErr := common.Unmarshal(responseBody, &errorResponse)
		if streamResponse.StatusCode == http.StatusGone || (unmarshalErr == nil && isCursorStreamUnavailable(errorResponse.Error.Code)) {
			run, waitErr := a.waitCursorRun(c, info, agentID, runID)
			if waitErr != nil {
				a.finishCursorRun(c, info, agentID, runID, persistent, false)
				return nil, waitErr
			}
			streamResponse, err = cursorRunFallbackResponse(run)
			if err != nil {
				a.finishCursorRun(c, info, agentID, runID, persistent, true)
				return nil, err
			}
		} else {
			streamResponse.Body = io.NopCloser(bytes.NewReader(responseBody))
		}
	}
	streamResponse.Header.Set(cursorAgentIDInternalHeader, agentID)
	streamResponse.Header.Set(cursorRunIDInternalHeader, runID)
	streamResponse.Header.Set(cursorPersistentInternalKey, strconv.FormatBool(persistent))
	streamResponse.Header.Set(cursorClientStreamHeader, strconv.FormatBool(info.IsStream))
	if creatingPersistentAgent {
		streamResponse.Header.Set(cursorAgentLifecycleHeader, constant.CursorAgentLifecycleCreate)
	}
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

func isCursorStreamUnavailable(code string) bool {
	switch strings.ToLower(strings.TrimSpace(code)) {
	case "stream_unavailable", "stream_expired":
		return true
	default:
		return false
	}
}

func cursorRunTerminal(status string) bool {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "FINISHED", "ERROR", "CANCELLED", "EXPIRED":
		return true
	default:
		return false
	}
}

func cursorRunFallbackResponse(run *cursorRunResponse) (*http.Response, error) {
	if run == nil {
		return nil, errors.New("cursor channel: empty run fallback response")
	}
	resultData, err := common.Marshal(cursorResultEvent{
		RunID:      run.ID,
		Status:     run.Status,
		Text:       run.Result,
		DurationMS: run.DurationMS,
	})
	if err != nil {
		return nil, fmt.Errorf("cursor channel: encode run fallback response: %w", err)
	}
	body := "event: result\ndata: " + string(resultData) + "\n\nevent: done\ndata: {}\n\n"
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}

func (a *Adaptor) waitCursorRun(c *gin.Context, info *relaycommon.RelayInfo, agentID string, runID string) (*cursorRunResponse, error) {
	if c == nil || c.Request == nil {
		return nil, errors.New("cursor channel: request context is required while waiting for a run")
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), cursorRunPollTimeout)
	defer cancel()
	runPath := "/v1/agents/" + url.PathEscape(agentID) + "/runs/" + url.PathEscape(runID)
	pollCount := 0
	for {
		response, err := a.doCursorAPIRequest(ctx, c, info, http.MethodGet, runPath, nil, false)
		if err != nil {
			return nil, fmt.Errorf("cursor channel: get run after stream became unavailable: %w", err)
		}
		responseBody, readErr := io.ReadAll(response.Body)
		service.CloseResponseBodyGracefully(response)
		if readErr != nil {
			return nil, fmt.Errorf("cursor channel: read run fallback response: %w", readErr)
		}
		if response.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("cursor channel: get run after stream became unavailable returned status %d", response.StatusCode)
		}
		var run cursorRunResponse
		if err := common.Unmarshal(responseBody, &run); err != nil {
			return nil, fmt.Errorf("cursor channel: decode run fallback response: %w", err)
		}
		if run.ID == "" || run.Status == "" {
			return nil, errors.New("cursor channel: run fallback response is missing id or status")
		}
		if cursorRunTerminal(run.Status) {
			return &run, nil
		}

		pollCount++
		if info != nil && info.IsStream && pollCount%10 == 0 {
			helper.SetEventStreamHeaders(c)
			if err := helper.PingData(c); err != nil {
				return nil, err
			}
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("cursor channel: wait for run after stream became unavailable: %w", ctx.Err())
		case <-time.After(cursorRunPollInterval):
		}
	}
}

func (a *Adaptor) DeletePersistentAgent(c *gin.Context, info *relaycommon.RelayInfo, agentID string) error {
	agentID = strings.TrimSpace(agentID)
	if err := validateCursorAgentChannel(c, info); err != nil {
		return err
	}
	if err := validateCursorAgentID(c, info, agentID); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	deletePath := "/v1/agents/" + url.PathEscape(agentID)
	response, err := a.doCursorAPIRequest(ctx, c, info, http.MethodDelete, deletePath, nil, false)
	if err != nil {
		return err
	}
	defer service.CloseResponseBodyGracefully(response)
	if response.StatusCode == http.StatusNotFound {
		return nil
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("cursor channel: delete agent returned status %d", response.StatusCode)
	}
	return nil
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

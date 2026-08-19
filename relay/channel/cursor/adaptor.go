package cursor

import (
	"bytes"
	"context"
	"crypto/hmac"
	"encoding/base64"
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
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tidwall/sjson"
	"golang.org/x/net/http/httpguts"
)

type Adaptor struct{}

func (a *Adaptor) Init(_ *relaycommon.RelayInfo) {}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if info == nil {
		return "", errors.New("cursor channel: relay info is required")
	}
	if info.RelayMode == relayconstant.RelayModeResponsesCompact {
		return "", errors.New("cursor channel: /v1/responses/compact endpoint not supported")
	}
	if info.RelayFormat != types.RelayFormatClaude &&
		info.RelayFormat != types.RelayFormatOpenAIResponses &&
		info.RelayMode != 0 &&
		info.RelayMode != relayconstant.RelayModeChatCompletions &&
		info.RelayMode != relayconstant.RelayModeResponses {
		return "", errors.New("cursor channel: only /v1/chat/completions, /v1/messages and /v1/responses are supported")
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
	authorization := "Bearer " + key
	if !httpguts.ValidHeaderFieldValue(authorization) {
		return types.NewError(
			errors.New("cursor channel: API key contains invalid header characters"),
			types.ErrorCodeChannelInvalidKey,
		)
	}

	channel.SetupApiRequestHeader(info, c, header)
	header.Set("Authorization", authorization)
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
	if len(request.Functions) > 0 || len(request.FunctionCall) > 0 {
		return nil, errors.New("cursor channel: legacy functions are not supported; use tools instead")
	}

	modelSelection := cursorModelSelection{ID: request.Model}
	reasoningEffort := strings.TrimSpace(request.ReasoningEffort)
	if reasoningEffort != "" {
		if info != nil {
			info.SetReasoningEffort(reasoningEffort)
		}
	}

	externalToolSpecs := make(map[string]cursorExternalToolSpec, len(request.Tools))
	externalToolsPrompt := ""
	if len(request.Tools) > 0 {
		// Cursor Cloud Agents accept a text prompt rather than client-defined tool schemas,
		// so expose those tools through a small, validated text protocol.
		externalTools := make([]any, 0, len(request.Tools))
		for index := range request.Tools {
			tool := &request.Tools[index]
			toolType := strings.TrimSpace(tool.Type)
			if toolType == "" {
				toolType = "function"
			}
			name := strings.TrimSpace(tool.Function.Name)
			alias := cursorExternalToolAliasPrefix + strconv.Itoa(len(externalTools)+1)
			switch toolType {
			case "function":
				toolCopy := *tool
				toolCopy.Function.Name = alias
				externalTools = append(externalTools, toolCopy)
			case dto.CustomType:
				var customTool map[string]any
				if err := common.Unmarshal(tool.Custom, &customTool); err != nil {
					return nil, fmt.Errorf("cursor channel: invalid custom tool: %w", err)
				}
				if customName, ok := customTool["name"].(string); ok {
					name = strings.TrimSpace(customName)
				}
				customTool["name"] = alias
				externalTools = append(externalTools, customTool)
			default:
				continue
			}
			if name == "" {
				return nil, errors.New("cursor channel: tool name is required")
			}
			spec := cursorExternalToolSpec{Kind: toolType, Name: name}
			externalToolSpecs[alias] = spec
			// Accept the original name as a compatibility fallback for models that
			// remember it from the conversation, but only advertise the alias.
			if _, exists := externalToolSpecs[name]; !exists {
				externalToolSpecs[name] = spec
			}
		}
		toolsJSON, err := common.Marshal(externalTools)
		if err != nil {
			return nil, fmt.Errorf("cursor channel: encode tools: %w", err)
		}
		var toolChoiceJSON []byte
		if request.ToolChoice != nil {
			toolChoiceJSON, err = common.Marshal(request.ToolChoice)
			if err != nil {
				return nil, fmt.Errorf("cursor channel: encode tool choice: %w", err)
			}
		}
		var protocol strings.Builder
		protocol.WriteString("\nThe client exposes external tools that run in the client's own environment and can access its files. ")
		protocol.WriteString("The listed tool names are client-only aliases. Never invoke Cursor's internal Agent, subagent, repository, shell, file, or artifact tools, even when they appear equivalent. ")
		protocol.WriteString("Cursor's /agent directory is a remote cloud workspace and is never evidence about the client's files. Any request to inspect, search, edit, build, or run the client's code must use the relevant client external tool. ")
		protocol.WriteString("Do not claim that client-local files are unavailable before requesting the relevant external tools. ")
		protocol.WriteString("When an external tool is needed, return only one valid JSON object with no preamble, apology, explanation, Markdown fence, or trailing text. JSON-escape all quotes, newlines, and other string characters. For function tools use this exact shape: ")
		protocol.WriteString(`{"cursor_external_tool_calls":[{"id":"call_unique","name":"tool_name","arguments":{}}]}`)
		protocol.WriteString(". For custom tools use this exact shape with free-form string input: ")
		protocol.WriteString(`{"cursor_external_tool_calls":[{"id":"call_unique","name":"tool_name","input":"free-form tool input"}]}`)
		protocol.WriteString(". The name must match an advertised client-only alias exactly. Multiple calls may be returned in the array, using arguments only for function tools and input only for custom tools. ")
		protocol.WriteString("If you accidentally used a Cursor internal tool, do not report its result or apologize; request the intended client external tool with this JSON envelope instead. ")
		protocol.WriteString("If no external tool is needed, return the normal assistant answer without this JSON envelope.\nAvailable external tools: ")
		protocol.Write(toolsJSON)
		if len(toolChoiceJSON) > 0 {
			protocol.WriteString("\nClient tool choice: ")
			protocol.Write(toolChoiceJSON)
		}
		externalToolsPrompt = protocol.String()
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
			if cacheErr := service.DeleteCursorAgentSession(c); cacheErr != nil {
				logger.LogWarn(c, "cursor channel: clear invalid agent session failed: "+cacheErr.Error())
			}
			metadata.AgentID = ""
		}
	}

	transcriptMessages := make([]cursorTranscriptMessage, 0, len(request.Messages))
	for index := range request.Messages {
		message := &request.Messages[index]
		switch message.Role {
		case "system", "developer", "user", "assistant", "tool":
		default:
			return nil, fmt.Errorf("cursor channel: message role %q is not supported in text chat mode", message.Role)
		}
		transcriptMessage := cursorTranscriptMessage{Role: message.Role, ToolCallID: message.ToolCallId}
		if message.Name != nil {
			transcriptMessage.Name = *message.Name
		}
		if message.Content != nil {
			content, images, err := contentForCursor(message)
			if err != nil {
				return nil, cursorInvalidRequest(err)
			}
			transcriptMessage.Content = content
			transcriptMessage.Images = images
		}
		for _, toolCall := range message.ParseToolCalls() {
			arguments := any(map[string]any{})
			if strings.TrimSpace(toolCall.Function.Arguments) != "" {
				if err := common.UnmarshalJsonStr(toolCall.Function.Arguments, &arguments); err != nil {
					arguments = toolCall.Function.Arguments
				}
			}
			transcriptMessage.ToolCalls = append(transcriptMessage.ToolCalls, cursorTranscriptToolCall{
				ID:        toolCall.ID,
				Type:      toolCall.Type,
				Name:      toolCall.Function.Name,
				Arguments: arguments,
			})
		}
		if transcriptMessage.Content == "" && len(transcriptMessage.ToolCalls) == 0 && transcriptMessage.ToolCallID == "" {
			return nil, errors.New("cursor channel: message must contain text or a tool call")
		}
		transcriptMessages = append(transcriptMessages, transcriptMessage)
	}

	if c != nil {
		c.Set(cursorAgentIDContextKey, metadata.AgentID)
		c.Set(cursorPersistentContextKey, metadata.Persistent)
		c.Set(cursorExternalToolsContextKey, externalToolSpecs)
	}

	if metadata.AgentID != "" {
		latest := transcriptMessages[len(transcriptMessages)-1]
		if latest.Role == "user" && strings.TrimSpace(latest.Content) == cursorCleanupCommand {
			common.SetContextKey(c, constant.ContextKeyCursorAgentLifecycle, constant.CursorAgentLifecycleDelete)
			return &deleteAgentRequest{}, nil
		}

		continuationStart := 0
		for index := len(transcriptMessages) - 1; index >= 0; index-- {
			if transcriptMessages[index].Role == "assistant" {
				continuationStart = index + 1
				break
			}
		}
		continuation := transcriptMessages[continuationStart:]
		if len(continuation) == 0 {
			return nil, errors.New("cursor channel: persistent agent continuation has no new client message")
		}
		busyFallback, err := cursorCreateAgentRequest(transcriptMessages, externalToolsPrompt, modelSelection)
		if err != nil {
			return nil, err
		}
		if c != nil {
			c.Set(cursorBusyFallbackContextKey, busyFallback)
		}
		promptImages, err := cursorPromptImages(continuation)
		if err != nil {
			return nil, cursorInvalidRequest(err)
		}
		if len(continuation) == 1 && continuation[0].Role == "user" && len(continuation[0].ToolCalls) == 0 && continuation[0].ToolCallID == "" {
			return &createRunRequest{Prompt: cursorPrompt{
				Text:   continuation[0].Content + externalToolsPrompt,
				Images: promptImages,
			}}, nil
		}
		var prompt strings.Builder
		prompt.WriteString("Continue the conversation using these new client events as JSON Lines:\n")
		for _, message := range continuation {
			line, err := common.Marshal(message)
			if err != nil {
				return nil, fmt.Errorf("cursor channel: encode continuation: %w", err)
			}
			prompt.Write(line)
			prompt.WriteByte('\n')
		}
		prompt.WriteString("Respond to the new events.")
		prompt.WriteString(externalToolsPrompt)
		return &createRunRequest{Prompt: cursorPrompt{Text: prompt.String(), Images: promptImages}}, nil
	}
	latest := transcriptMessages[len(transcriptMessages)-1]
	if latest.Role == "user" && strings.TrimSpace(latest.Content) == cursorCleanupCommand {
		return nil, errors.New("cursor channel: no active Cursor Agent can be deleted")
	}
	if info != nil && info.IsChannelTest {
		return &createAgentRequest{
			Prompt: cursorPrompt{Text: "Reply exactly with OK. Do not use tools or inspect any repository."},
			Model:  modelSelection,
			Name:   "new-api channel test",
		}, nil
	}
	return cursorCreateAgentRequest(transcriptMessages, externalToolsPrompt, modelSelection)
}

func cursorCreateAgentRequest(
	messages []cursorTranscriptMessage,
	externalToolsPrompt string,
	model cursorModelSelection,
) (*createAgentRequest, error) {
	promptImages, err := cursorPromptImages(messages)
	if err != nil {
		return nil, cursorInvalidRequest(err)
	}

	var transcript strings.Builder
	transcript.WriteString("You are continuing a client chat. Do not use Cursor's repository, shell, file editing, or artifact tools.\n")
	transcript.WriteString("Conversation transcript follows as JSON Lines. Respect each message role and all prior context.\n")
	for _, message := range messages {
		line, err := common.Marshal(message)
		if err != nil {
			return nil, fmt.Errorf("cursor channel: encode conversation: %w", err)
		}
		transcript.Write(line)
		transcript.WriteByte('\n')
	}
	transcript.WriteString("Return only the assistant reply to the final client event.")
	transcript.WriteString(externalToolsPrompt)

	return &createAgentRequest{
		Prompt: cursorPrompt{Text: transcript.String(), Images: promptImages},
		Model:  model,
		Name:   "new-api text chat",
	}, nil
}

func cursorInvalidRequest(err error) error {
	return types.NewErrorWithStatusCode(
		err,
		types.ErrorCodeInvalidRequest,
		http.StatusBadRequest,
		types.ErrOptionWithSkipRetry(),
	)
}

func contentForCursor(message *dto.Message) (string, []cursorPromptImage, error) {
	if message == nil || message.Content == nil {
		return "", nil, errors.New("cursor channel: message content is required")
	}
	if content, ok := message.Content.(string); ok {
		return content, nil, nil
	}

	var content strings.Builder
	images := make([]cursorPromptImage, 0)

	switch parts := message.Content.(type) {
	case []dto.MediaContent:
		for _, part := range parts {
			var err error
			images, err = appendCursorContentPart(&content, images, part)
			if err != nil {
				return "", nil, err
			}
		}
	case []any:
		for _, rawPart := range parts {
			var err error
			images, err = appendCursorContentPart(&content, images, rawPart)
			if err != nil {
				return "", nil, err
			}
		}
	case []map[string]any:
		for _, part := range parts {
			var err error
			images, err = appendCursorContentPart(&content, images, part)
			if err != nil {
				return "", nil, err
			}
		}
	case dto.MediaContent, *dto.MediaContent, map[string]any:
		var err error
		images, err = appendCursorContentPart(&content, images, parts)
		if err != nil {
			return "", nil, err
		}
	default:
		return "", nil, fmt.Errorf("cursor channel: message content type %T is not supported", message.Content)
	}
	return content.String(), images, nil
}

func appendCursorContentPart(content *strings.Builder, images []cursorPromptImage, rawPart any) ([]cursorPromptImage, error) {
	switch part := rawPart.(type) {
	case dto.MediaContent:
		partType := strings.TrimSpace(part.Type)
		switch partType {
		case dto.ContentTypeText, "input_text", "output_text":
			content.WriteString(part.Text)
			return images, nil
		case dto.ContentTypeImageURL, "input_image":
			image, err := cursorPromptImageFromValue(part.ImageUrl, "")
			if err != nil {
				return nil, err
			}
			content.WriteString("\n[Image attached to this message]\n")
			return append(images, image), nil
		default:
			return nil, fmt.Errorf("cursor channel: message content type %q is not supported", partType)
		}
	case *dto.MediaContent:
		if part == nil {
			return nil, errors.New("cursor channel: message content part is empty")
		}
		return appendCursorContentPart(content, images, *part)
	case map[string]any:
		partType, _ := part["type"].(string)
		partType = strings.TrimSpace(partType)
		switch partType {
		case dto.ContentTypeText, "input_text", "output_text":
			text, ok := part["text"].(string)
			if !ok {
				return nil, fmt.Errorf("cursor channel: %s content requires a text string", partType)
			}
			content.WriteString(text)
			return images, nil
		case "refusal":
			refusal, ok := part["refusal"].(string)
			if !ok {
				return nil, errors.New("cursor channel: refusal content requires a refusal string")
			}
			content.WriteString(refusal)
			return images, nil
		case dto.ContentTypeImageURL, "input_image":
			imageValue := part["image_url"]
			if imageValue == nil {
				imageValue = part["url"]
			}
			image, err := cursorPromptImageFromValue(imageValue, "")
			if err != nil {
				return nil, err
			}
			content.WriteString("\n[Image attached to this message]\n")
			return append(images, image), nil
		default:
			return nil, fmt.Errorf("cursor channel: message content type %q is not supported", partType)
		}
	default:
		return nil, fmt.Errorf("cursor channel: message content part type %T is not supported", rawPart)
	}
}

func cursorPromptImageFromValue(value any, mimeType string) (cursorPromptImage, error) {
	switch image := value.(type) {
	case string:
		return cursorPromptImageFromSource(image, mimeType)
	case dto.MessageImageUrl:
		return cursorPromptImageFromSource(image.Url, image.MimeType)
	case *dto.MessageImageUrl:
		if image == nil {
			return cursorPromptImage{}, errors.New("cursor channel: image content is empty")
		}
		return cursorPromptImageFromSource(image.Url, image.MimeType)
	case map[string]any:
		if data, ok := image["data"].(string); ok && strings.TrimSpace(data) != "" {
			resolvedMimeType, _ := image["mimeType"].(string)
			if resolvedMimeType == "" {
				resolvedMimeType, _ = image["mime_type"].(string)
			}
			return cursorPromptImageFromSource(data, resolvedMimeType)
		}
		urlValue, _ := image["url"].(string)
		resolvedMimeType, _ := image["mime_type"].(string)
		return cursorPromptImageFromSource(urlValue, resolvedMimeType)
	default:
		return cursorPromptImage{}, fmt.Errorf("cursor channel: image content type %T is not supported", value)
	}
}

func cursorPromptImageFromSource(source string, mimeType string) (cursorPromptImage, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return cursorPromptImage{}, errors.New("cursor channel: image URL or data is required")
	}
	parsedURL, err := url.Parse(source)
	if err == nil && (parsedURL.Scheme == "http" || parsedURL.Scheme == "https") && parsedURL.Host != "" {
		return cursorPromptImage{URL: source}, nil
	}

	data := source
	if strings.HasPrefix(strings.ToLower(source), "data:") {
		header, encoded, found := strings.Cut(source, ",")
		if !found {
			return cursorPromptImage{}, errors.New("cursor channel: image data URL is invalid")
		}
		headerParts := strings.Split(header[len("data:"):], ";")
		mimeType = headerParts[0]
		base64Encoded := false
		for _, parameter := range headerParts[1:] {
			if strings.EqualFold(strings.TrimSpace(parameter), "base64") {
				base64Encoded = true
				break
			}
		}
		if !base64Encoded {
			return cursorPromptImage{}, errors.New("cursor channel: image data URL must be base64 encoded")
		}
		data = encoded
	}

	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	if mimeType == "image/jpg" {
		mimeType = "image/jpeg"
	}
	switch mimeType {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
	default:
		return cursorPromptImage{}, fmt.Errorf("cursor channel: image MIME type %q is not supported", mimeType)
	}
	data = strings.NewReplacer("\r", "", "\n", "").Replace(strings.TrimSpace(data))
	decodedSize, err := io.Copy(io.Discard, io.LimitReader(base64.NewDecoder(base64.StdEncoding, strings.NewReader(data)), cursorMaxImageBytes+1))
	if err != nil {
		return cursorPromptImage{}, fmt.Errorf("cursor channel: image data is not valid base64: %w", err)
	}
	if decodedSize > cursorMaxImageBytes {
		return cursorPromptImage{}, fmt.Errorf("cursor channel: image exceeds the %d MB Cursor limit", cursorMaxImageBytes/(1024*1024))
	}
	return cursorPromptImage{Data: data, MimeType: mimeType}, nil
}

func cursorPromptImages(messages []cursorTranscriptMessage) ([]cursorPromptImage, error) {
	images := make([]cursorPromptImage, 0)
	for _, message := range messages {
		images = append(images, message.Images...)
		if len(images) > cursorMaxPromptImages {
			return nil, fmt.Errorf("cursor channel: Cursor accepts at most %d images per prompt", cursorMaxPromptImages)
		}
	}
	return images, nil
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
	releaseSerialExecution, err := acquireCursorAgentSerialExecution(c, info)
	if err != nil {
		return nil, fmt.Errorf("cursor channel: wait for previous Agent run: %w", err)
	}
	defer func() {
		if releaseSerialExecution != nil {
			releaseSerialExecution()
		}
	}()
	transferSerialRelease := func(response *http.Response) {
		if releaseSerialExecution == nil {
			return
		}
		transferCursorAgentSerialRelease(response, releaseSerialExecution)
		releaseSerialExecution = nil
	}
	if common.GetContextKeyString(c, constant.ContextKeyCursorAgentLifecycle) == constant.CursorAgentLifecycleDelete {
		agentID := c.GetString(cursorAgentIDContextKey)
		if err := a.DeletePersistentAgent(c, info, agentID); err != nil {
			return nil, err
		}
		if err := service.DeleteCursorAgentSession(c); err != nil {
			logger.LogWarn(c, "cursor channel: delete agent session cache failed: "+err.Error())
		}
		c.Header(constant.CursorAgentDeletedHeader, "true")
		responseHeader := make(http.Header)
		responseHeader.Set(cursorAgentLifecycleHeader, constant.CursorAgentLifecycleDelete)
		responseHeader.Set(cursorClientStreamHeader, strconv.FormatBool(info.IsStream))
		response := &http.Response{
			StatusCode: http.StatusOK,
			Header:     responseHeader,
			Body:       io.NopCloser(strings.NewReader("")),
		}
		transferSerialRelease(response)
		return response, nil
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
	} else {
		body, err = a.resolveCursorCreateAgentModel(c, info, body)
		if err != nil {
			return nil, err
		}
	}

	response, err := a.doCursorAPIRequest(c.Request.Context(), c, info, http.MethodPost, requestPath, bytes.NewReader(body), false)
	if err != nil {
		return nil, err
	}
	if agentID != "" && persistent && response.StatusCode == http.StatusConflict {
		responseBody, readErr := io.ReadAll(response.Body)
		service.CloseResponseBodyGracefully(response)
		if readErr != nil {
			return nil, fmt.Errorf("cursor channel: read create run error response: %w", readErr)
		}
		var errorResponse cursorAPIErrorResponse
		unmarshalErr := common.Unmarshal(responseBody, &errorResponse)
		busy := unmarshalErr == nil && (strings.EqualFold(strings.TrimSpace(errorResponse.Error.Code), cursorAgentBusyCode) ||
			strings.EqualFold(strings.TrimSpace(errorResponse.Error.Message), "Agent already has an active run"))
		fallbackValue, fallbackExists := c.Get(cursorBusyFallbackContextKey)
		fallback, fallbackValid := fallbackValue.(*createAgentRequest)
		if busy && !info.ChannelSetting.CursorAgentSerialExecution && fallbackExists && fallbackValid && fallback != nil {
			body, err = common.Marshal(fallback)
			if err != nil {
				return nil, fmt.Errorf("cursor channel: encode busy Agent fallback: %w", err)
			}
			body, err = a.resolveCursorCreateAgentModel(c, info, body)
			if err != nil {
				return nil, err
			}
			logger.LogWarn(c, "cursor channel: persistent Agent already has an active run; use one ephemeral Agent for this request")
			agentID = ""
			persistent = false
			creatingPersistentAgent = false
			response, err = a.doCursorAPIRequest(c.Request.Context(), c, info, http.MethodPost, "/v1/agents", bytes.NewReader(body), false)
			if err != nil {
				return nil, err
			}
		} else {
			response.Body = io.NopCloser(bytes.NewReader(responseBody))
		}
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		transferSerialRelease(response)
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
		signature := cursorAgentSignature(c.GetInt("id"), agentID, info.ChannelId, info.ChannelMultiKeyIndex, info.ApiKey)
		if err := service.SaveCursorAgentSession(c, service.CursorAgentSession{
			AgentID:   agentID,
			Signature: signature,
			ChannelID: info.ChannelId,
			KeyIndex:  info.ChannelMultiKeyIndex,
		}); err != nil {
			logger.LogWarn(c, "cursor channel: save agent session cache failed: "+err.Error())
		}
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
	transferSerialRelease(streamResponse)
	return streamResponse, nil
}

func (a *Adaptor) startCursorExternalToolRecoveryRun(
	c *gin.Context,
	info *relaycommon.RelayInfo,
	agentID string,
	internalToolName string,
	clientToolAlias string,
	persistent bool,
	clientStream bool,
	skipRemoteUsage bool,
) (*http.Response, error) {
	if c == nil || c.Request == nil {
		return nil, errors.New("cursor channel: request context is required for external tool recovery")
	}
	internalToolName = strings.TrimSpace(internalToolName)
	recoveryPrompt := fmt.Sprintf(
		"The previous run inspected Cursor's remote cloud workspace instead of the client's workspace. Do not use or retry any Cursor internal tools. Re-read the client external-tool protocol and schema from the preceding prompt, then request the matching client tool alias %q. Return only the valid cursor_external_tool_calls JSON object with fully escaped string values and no other text.",
		clientToolAlias,
	)
	if internalToolName != "" {
		recoveryPrompt = fmt.Sprintf(
			"The previous run incorrectly invoked Cursor's internal %q tool against the remote cloud workspace. Do not use or retry any Cursor internal tools. Re-read the client external-tool protocol and schema from the preceding prompt, then request the matching client tool alias %q. Return only the valid cursor_external_tool_calls JSON object with fully escaped string values and no other text.",
			internalToolName,
			clientToolAlias,
		)
	}
	body, err := common.Marshal(createRunRequest{Prompt: cursorPrompt{Text: recoveryPrompt}})
	if err != nil {
		return nil, fmt.Errorf("cursor channel: encode external tool recovery run: %w", err)
	}
	createPath := "/v1/agents/" + url.PathEscape(agentID) + "/runs"
	createResponse, err := a.doCursorAPIRequest(c.Request.Context(), c, info, http.MethodPost, createPath, bytes.NewReader(body), false)
	if err != nil {
		return nil, err
	}
	createBody, readErr := io.ReadAll(createResponse.Body)
	service.CloseResponseBodyGracefully(createResponse)
	if readErr != nil {
		return nil, fmt.Errorf("cursor channel: read external tool recovery response: %w", readErr)
	}
	if createResponse.StatusCode < http.StatusOK || createResponse.StatusCode >= http.StatusMultipleChoices {
		var errorResponse cursorAPIErrorResponse
		if err := common.Unmarshal(createBody, &errorResponse); err == nil && errorResponse.Error.Message != "" {
			return nil, fmt.Errorf("cursor channel: create external tool recovery run returned status %d: %s", createResponse.StatusCode, errorResponse.Error.Message)
		}
		return nil, fmt.Errorf("cursor channel: create external tool recovery run returned status %d", createResponse.StatusCode)
	}
	var created createRunResponse
	if err := common.Unmarshal(createBody, &created); err != nil {
		return nil, fmt.Errorf("cursor channel: decode external tool recovery response: %w", err)
	}
	runID := strings.TrimSpace(created.Run.ID)
	if runID == "" {
		return nil, errors.New("cursor channel: external tool recovery response is missing run ID")
	}
	handedOff := false
	defer func() {
		if !handedOff {
			// The caller still owns the original Agent cleanup. Only cancel the
			// newly-created recovery run here so a stream setup failure cannot
			// leave it running on a persistent Agent.
			a.finishCursorRun(c, info, agentID, runID, true, false)
		}
	}()

	streamPath := "/v1/agents/" + url.PathEscape(agentID) + "/runs/" + url.PathEscape(runID) + "/stream"
	streamResponse, err := a.doCursorAPIRequest(c.Request.Context(), c, info, http.MethodGet, streamPath, nil, true)
	if err != nil {
		return nil, err
	}
	if streamResponse.StatusCode != http.StatusOK {
		responseBody, readErr := io.ReadAll(streamResponse.Body)
		service.CloseResponseBodyGracefully(streamResponse)
		if readErr != nil {
			return nil, fmt.Errorf("cursor channel: read external tool recovery stream response: %w", readErr)
		}
		var errorResponse cursorAPIErrorResponse
		unmarshalErr := common.Unmarshal(responseBody, &errorResponse)
		if streamResponse.StatusCode == http.StatusGone || (unmarshalErr == nil && isCursorStreamUnavailable(errorResponse.Error.Code)) {
			run, waitErr := a.waitCursorRun(c, info, agentID, runID)
			if waitErr != nil {
				return nil, waitErr
			}
			streamResponse, err = cursorRunFallbackResponse(run)
			if err != nil {
				return nil, err
			}
		} else if unmarshalErr == nil && errorResponse.Error.Message != "" {
			return nil, fmt.Errorf("cursor channel: external tool recovery stream returned status %d: %s", streamResponse.StatusCode, errorResponse.Error.Message)
		} else {
			return nil, fmt.Errorf("cursor channel: external tool recovery stream returned status %d", streamResponse.StatusCode)
		}
	}
	streamResponse.Header.Set(cursorAgentIDInternalHeader, agentID)
	streamResponse.Header.Set(cursorRunIDInternalHeader, runID)
	streamResponse.Header.Set(cursorPersistentInternalKey, strconv.FormatBool(persistent))
	streamResponse.Header.Set(cursorClientStreamHeader, strconv.FormatBool(clientStream))
	if skipRemoteUsage {
		streamResponse.Header.Set(cursorSkipRemoteUsageHeader, "true")
	}
	handedOff = true
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
		Error:      run.Error,
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
			return nil, fmt.Errorf("cursor channel: get run: %w", err)
		}
		responseBody, readErr := io.ReadAll(response.Body)
		service.CloseResponseBodyGracefully(response)
		if readErr != nil {
			return nil, fmt.Errorf("cursor channel: read run response: %w", readErr)
		}
		if response.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("cursor channel: get run returned status %d", response.StatusCode)
		}
		var run cursorRunResponse
		if err := common.Unmarshal(responseBody, &run); err != nil {
			return nil, fmt.Errorf("cursor channel: decode run response: %w", err)
		}
		if run.ID == "" || run.Status == "" {
			return nil, errors.New("cursor channel: run response is missing id or status")
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
			return nil, fmt.Errorf("cursor channel: wait for run: %w", ctx.Err())
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

func (a *Adaptor) resolveCursorCreateAgentModel(c *gin.Context, info *relaycommon.RelayInfo, body []byte) ([]byte, error) {
	requestedEffort := ""
	if info != nil {
		requestedEffort = info.GetReasoningEffort()
	}
	if requestedEffort == "" {
		return body, nil
	}

	var request struct {
		Model cursorModelSelection `json:"model"`
	}
	if err := common.Unmarshal(body, &request); err != nil {
		return nil, fmt.Errorf("cursor channel: decode create agent model: %w", err)
	}

	selection, applied, err := a.resolveCursorModelSelection(c.Request.Context(), c, info, request.Model, requestedEffort)
	if err != nil {
		return nil, err
	}
	if !applied && info != nil {
		info.SetReasoningEffort("")
	}
	modelJSON, err := common.Marshal(selection)
	if err != nil {
		return nil, fmt.Errorf("cursor channel: encode resolved model selection: %w", err)
	}
	resolvedBody, err := sjson.SetRawBytes(body, "model", modelJSON)
	if err != nil {
		return nil, fmt.Errorf("cursor channel: replace resolved model selection: %w", err)
	}
	return resolvedBody, nil
}

func (a *Adaptor) resolveCursorModelSelection(
	ctx context.Context,
	c *gin.Context,
	info *relaycommon.RelayInfo,
	selection cursorModelSelection,
	requestedEffort string,
) (cursorModelSelection, bool, error) {
	response, err := a.doCursorAPIRequest(ctx, c, info, http.MethodGet, "/v1/models", nil, false)
	if err != nil {
		if ctx.Err() != nil {
			return cursorModelSelection{}, false, err
		}
		logger.LogWarn(c, "cursor channel: model catalog unavailable; omit unsupported reasoning params: "+err.Error())
		return selection, false, nil
	}
	defer service.CloseResponseBodyGracefully(response)
	if response.StatusCode != http.StatusOK {
		logger.LogWarn(c, fmt.Sprintf("cursor channel: model catalog returned status %d; omit unsupported reasoning params", response.StatusCode))
		return selection, false, nil
	}

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		if ctx.Err() != nil {
			return cursorModelSelection{}, false, fmt.Errorf("cursor channel: read model catalog: %w", err)
		}
		logger.LogWarn(c, "cursor channel: cannot read model catalog; omit unsupported reasoning params: "+err.Error())
		return selection, false, nil
	}
	var catalog cursorModelCatalogResponse
	if err := common.Unmarshal(responseBody, &catalog); err != nil {
		logger.LogWarn(c, "cursor channel: invalid model catalog; omit unsupported reasoning params: "+err.Error())
		return selection, false, nil
	}

	var item *cursorModelCatalogItem
	requestedModel := strings.TrimSpace(selection.ID)
	for index := range catalog.Items {
		candidate := &catalog.Items[index]
		if strings.EqualFold(strings.TrimSpace(candidate.ID), requestedModel) {
			item = candidate
			break
		}
		for _, alias := range candidate.Aliases {
			if strings.EqualFold(strings.TrimSpace(alias), requestedModel) {
				item = candidate
				break
			}
		}
		if item != nil {
			break
		}
	}
	if item == nil {
		logger.LogWarn(c, fmt.Sprintf("cursor channel: model %q is absent from the model catalog; omit unsupported reasoning params", requestedModel))
		return selection, false, nil
	}

	defaultSelection := cursorModelSelection{ID: strings.TrimSpace(item.ID)}
	reasoningParamValues := make(map[string]string)
	for _, parameter := range item.Parameters {
		identity := strings.ToLower(strings.TrimSpace(parameter.ID + " " + parameter.DisplayName))
		if !strings.Contains(identity, "think") && !strings.Contains(identity, "reason") && !strings.Contains(identity, "effort") {
			continue
		}
		for _, value := range parameter.Values {
			if strings.EqualFold(strings.TrimSpace(value.Value), requestedEffort) {
				reasoningParamValues[strings.ToLower(strings.TrimSpace(parameter.ID))] = strings.TrimSpace(value.Value)
				break
			}
		}
	}

	var matchedVariant *cursorModelVariant
	for index := range item.Variants {
		variant := &item.Variants[index]
		matchesEffort := false
		for _, param := range variant.Params {
			paramID := strings.ToLower(strings.TrimSpace(param.ID))
			catalogValue, ok := reasoningParamValues[paramID]
			if ok && strings.EqualFold(strings.TrimSpace(param.Value), catalogValue) {
				matchesEffort = true
				break
			}
			if !ok && (strings.Contains(paramID, "think") || strings.Contains(paramID, "reason") || strings.Contains(paramID, "effort")) &&
				strings.EqualFold(strings.TrimSpace(param.Value), requestedEffort) {
				matchesEffort = true
				break
			}
		}
		if !matchesEffort {
			continue
		}
		if matchedVariant == nil || len(variant.Params) < len(matchedVariant.Params) ||
			(len(variant.Params) == len(matchedVariant.Params) && variant.IsDefault && !matchedVariant.IsDefault) {
			matchedVariant = variant
		}
	}
	if matchedVariant != nil {
		return cursorModelSelection{ID: defaultSelection.ID, Params: matchedVariant.Params}, true, nil
	}

	if len(item.Variants) == 0 {
		for _, parameter := range item.Parameters {
			catalogValue, ok := reasoningParamValues[strings.ToLower(strings.TrimSpace(parameter.ID))]
			if ok {
				return cursorModelSelection{
					ID:     defaultSelection.ID,
					Params: []cursorModelParam{{ID: strings.TrimSpace(parameter.ID), Value: catalogValue}},
				}, true, nil
			}
		}
	}

	logger.LogWarn(c, fmt.Sprintf("cursor channel: model %q has no exact reasoning effort %q variant; use its catalog default", defaultSelection.ID, requestedEffort))
	return defaultSelection, false, nil
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

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	if info != nil && info.RelayMode == relayconstant.RelayModeResponsesCompact {
		return nil, errors.New("cursor channel: /v1/responses/compact endpoint not supported")
	}
	externalToolSpecs, err := cursorResponsesExternalToolSpecs(request)
	if err != nil {
		return nil, err
	}
	openAIRequest, err := service.ResponsesRequestToChatCompletionsRequest(&request)
	if err != nil {
		return nil, fmt.Errorf("cursor channel: convert Responses request: %w", err)
	}
	converted, err := a.ConvertOpenAIRequest(c, info, openAIRequest)
	if err == nil && c != nil && len(externalToolSpecs) > 0 {
		convertedSpecs := cursorExternalToolSpecs(c)
		if convertedSpecs == nil {
			convertedSpecs = make(map[string]cursorExternalToolSpec, len(externalToolSpecs))
		}
		for key, spec := range convertedSpecs {
			qualifiedName := cursorQualifiedToolName(spec.Namespace, spec.Name)
			if responseSpec, exists := externalToolSpecs[qualifiedName]; exists {
				convertedSpecs[key] = responseSpec
			}
		}
		for key, spec := range externalToolSpecs {
			convertedSpecs[key] = spec
		}
		c.Set(cursorExternalToolsContextKey, convertedSpecs)
	}
	return converted, err
}

func cursorResponsesExternalToolSpecs(request dto.OpenAIResponsesRequest) (map[string]cursorExternalToolSpec, error) {
	tools := make([]map[string]any, 0)
	if len(request.Tools) > 0 {
		if err := common.Unmarshal(request.Tools, &tools); err != nil {
			return nil, fmt.Errorf("cursor channel: invalid Responses tools: %w", err)
		}
	}
	if len(request.Input) > 0 && common.GetJsonType(request.Input) == "array" {
		var inputItems []map[string]any
		if err := common.Unmarshal(request.Input, &inputItems); err != nil {
			return nil, fmt.Errorf("cursor channel: invalid Responses input: %w", err)
		}
		for _, item := range inputItems {
			if itemType, _ := item["type"].(string); itemType != "additional_tools" {
				continue
			}
			additionalTools, ok := item["tools"].([]any)
			if !ok {
				continue
			}
			for _, rawTool := range additionalTools {
				if tool, ok := rawTool.(map[string]any); ok {
					tools = append(tools, tool)
				}
			}
		}
	}

	specs := make(map[string]cursorExternalToolSpec)
	for _, tool := range tools {
		appendCursorResponsesToolSpecs(specs, tool, "")
	}
	return specs, nil
}

func appendCursorResponsesToolSpecs(specs map[string]cursorExternalToolSpec, tool map[string]any, namespace string) {
	toolType, _ := tool["type"].(string)
	toolType = strings.TrimSpace(toolType)
	name, _ := tool["name"].(string)
	name = strings.TrimSpace(name)
	if toolType == "namespace" {
		namespace = cursorQualifiedToolName(namespace, name)
		children, ok := tool["tools"].([]any)
		if !ok {
			return
		}
		for _, rawChild := range children {
			if child, ok := rawChild.(map[string]any); ok {
				appendCursorResponsesToolSpecs(specs, child, namespace)
			}
		}
		return
	}
	if (toolType != "function" && toolType != dto.CustomType) || name == "" {
		return
	}
	qualifiedName := cursorQualifiedToolName(namespace, name)
	specs[qualifiedName] = cursorExternalToolSpec{Kind: toolType, Name: name, Namespace: namespace}
}

func cursorQualifiedToolName(namespace string, name string) string {
	if namespace == "" || name == "" || strings.HasPrefix(name, namespace+"__") {
		return name
	}
	if strings.HasSuffix(namespace, "__") {
		return namespace + name
	}
	return namespace + "__" + name
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) {
	result, err := service.ConvertRequest(c, info, types.RelayFormatOpenAI, request)
	if err != nil {
		return nil, err
	}
	openAIRequest, ok := result.Value.(*dto.GeneralOpenAIRequest)
	if !ok {
		return nil, fmt.Errorf("cursor channel: expected OpenAI chat completions request, got %T", result.Value)
	}
	openAIRequest.ReasoningEffort = request.GetEfforts()
	openAIRequest.Metadata = request.Metadata
	openAIRequest.ToolChoice = request.ToolChoice
	return a.ConvertOpenAIRequest(c, info, openAIRequest)
}

func (a *Adaptor) ConvertGeminiRequest(*gin.Context, *relaycommon.RelayInfo, *dto.GeminiChatRequest) (any, error) {
	return nil, errors.New("cursor channel: Gemini endpoint not supported")
}

var _ channel.Adaptor = (*Adaptor)(nil)

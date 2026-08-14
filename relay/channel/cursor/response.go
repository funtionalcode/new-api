package cursor

import (
	"bufio"
	"context"
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
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/relayconvert"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func scanCursorEvents(reader io.Reader, handle func(cursorEvent) error) error {
	scanner := helper.NewStreamScanner(reader)
	scanner.Split(bufio.ScanLines)

	eventType := ""
	dataLines := make([]string, 0, 1)
	dispatch := func() error {
		if eventType == "" && len(dataLines) == 0 {
			return nil
		}
		event := cursorEvent{Type: eventType, Data: []byte(strings.Join(dataLines, "\n"))}
		eventType = ""
		dataLines = dataLines[:0]
		return handle(event)
	}

	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		if line == "" {
			if err := dispatch(); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		field, value, found := strings.Cut(line, ":")
		if !found {
			field = line
			value = ""
		}
		value = strings.TrimPrefix(value, " ")
		switch field {
		case "event":
			eventType = value
		case "data":
			dataLines = append(dataLines, value)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return dispatch()
}

func cursorUsageToOpenAI(upstream cursorUsage, info *relaycommon.RelayInfo) *dto.Usage {
	inputTokens := cursorTokenCount(info, "inputTokens", upstream.InputTokens)
	outputTokens := cursorTokenCount(info, "outputTokens", upstream.OutputTokens)
	cacheWriteTokens := cursorTokenCount(info, "cacheWriteTokens", upstream.CacheWriteTokens)
	cacheReadTokens := cursorTokenCount(info, "cacheReadTokens", upstream.CacheReadTokens)
	promptTokens := cursorTokenTotal(info, inputTokens, cacheWriteTokens, cacheReadTokens)
	totalTokens := cursorTokenTotal(info, promptTokens, outputTokens)

	inputDetails := dto.InputTokenDetails{
		CachedTokens:     cacheReadTokens,
		CacheWriteTokens: cacheWriteTokens,
		TextTokens:       inputTokens,
	}
	usage := &dto.Usage{
		PromptTokens:         promptTokens,
		CompletionTokens:     outputTokens,
		TotalTokens:          totalTokens,
		PromptCacheHitTokens: cacheReadTokens,
		PromptTokensDetails:  inputDetails,
		CompletionTokenDetails: dto.OutputTokenDetails{
			TextTokens: outputTokens,
		},
		InputTokens:        promptTokens,
		OutputTokens:       outputTokens,
		InputTokensDetails: &inputDetails,
		UsageSemantic:      dto.BillingUsageSemanticOpenAI,
		UsageSource:        dto.BillingUsageSourceOAIChat,
	}
	usage.BillingUsage = dto.NewOpenAIChatBillingUsage(usage)
	return usage
}

func cursorTokenCount(info *relaycommon.RelayInfo, field string, value int64) int {
	if value < 0 {
		common.SysError(fmt.Sprintf("cursor usage %s is negative: %d", field, value))
		return 0
	}
	tokens, clamp := common.QuotaFromDecimalChecked(decimal.NewFromInt(value))
	if clamp != nil && info != nil && info.QuotaClamp == nil {
		info.QuotaClamp = clamp
	}
	return tokens
}

func cursorTokenTotal(info *relaycommon.RelayInfo, values ...int) int {
	total := decimal.Zero
	for _, value := range values {
		total = total.Add(decimal.NewFromInt(int64(value)))
	}
	tokens, clamp := common.QuotaFromDecimalChecked(total)
	if clamp != nil && info != nil && info.QuotaClamp == nil {
		info.QuotaClamp = clamp
	}
	return tokens
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (any, *types.NewAPIError) {
	return a.doResponse(c, resp, info, true, nil)
}

func (a *Adaptor) doResponse(
	c *gin.Context,
	resp *http.Response,
	info *relaycommon.RelayInfo,
	allowExternalToolRecovery bool,
	priorUsage *dto.Usage,
) (any, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewOpenAIError(errors.New("cursor channel: empty upstream response"), types.ErrorCodeBadResponse, http.StatusBadGateway)
	}
	defer service.CloseResponseBodyGracefully(resp)
	if resp.Header.Get(cursorAgentLifecycleHeader) == constant.CursorAgentLifecycleDelete {
		return writeCursorAgentDeletedResponse(c, info, resp)
	}

	agentID := resp.Header.Get(cursorAgentIDInternalHeader)
	runID := resp.Header.Get(cursorRunIDInternalHeader)
	persistent, _ := strconv.ParseBool(resp.Header.Get(cursorPersistentInternalKey))
	clientStream, _ := strconv.ParseBool(resp.Header.Get(cursorClientStreamHeader))
	if persistent && agentID != "" {
		setCursorAgentResponseHeaders(c, info, agentID)
	}
	if resp.Header.Get(cursorAgentLifecycleHeader) == constant.CursorAgentLifecycleCreate {
		common.SetContextKey(c, constant.ContextKeyCursorAgentLifecycle, constant.CursorAgentLifecycleCreate)
	}

	responseID := helper.GetResponseID(c)
	createdAt := common.GetTimestamp()
	model := info.UpstreamModelName
	externalToolNames, _ := c.Get(cursorExternalToolsContextKey)
	allowedExternalTools, _ := externalToolNames.(map[string]cursorExternalToolSpec)
	bufferAssistantForTools := len(allowedExternalTools) > 0
	var responsesStreamState *relayconvert.ResponseStreamState
	if clientStream && info.RelayFormat == types.RelayFormatOpenAIResponses {
		var err error
		responsesStreamState, err = relayconvert.NewResponseStreamState(types.RelayFormatOpenAI, types.RelayFormatOpenAIResponses, relayconvert.ResponseStreamOptions{
			ID:      responseID,
			Model:   model,
			Created: createdAt,
		})
		if err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
		}
	}
	var streamedText strings.Builder
	var streamedThinking strings.Builder
	finalText := ""
	resultStatus := ""
	upstreamError := cursorErrorEvent{}
	lastClientToolCollision := cursorToolCallEvent{}
	firstChunk := true
	thinkingStreamed := false
	clientOutputStarted := false

	if clientStream {
		helper.SetEventStreamHeaders(c)
	}

	parseErr := scanCursorEvents(resp.Body, func(event cursorEvent) error {
		if c.Request != nil && c.Request.Context().Err() != nil {
			return c.Request.Context().Err()
		}
		switch event.Type {
		case "assistant":
			var textEvent cursorTextEvent
			if err := common.Unmarshal(event.Data, &textEvent); err != nil {
				return fmt.Errorf("cursor channel: decode assistant event: %w", err)
			}
			if textEvent.Text == "" {
				return nil
			}
			streamedText.WriteString(textEvent.Text)
			if !clientStream {
				return nil
			}
			if firstChunk {
				info.FirstResponseTime = time.Now()
				firstChunk = false
			}
			chunkText := textEvent.Text
			if bufferAssistantForTools {
				if cursorExternalToolStreamCandidate(streamedText.String()) {
					return nil
				}
				bufferAssistantForTools = false
				chunkText = streamedText.String()
			}
			if !thinkingStreamed && streamedThinking.Len() > 0 {
				if err := writeCursorThinkingChunk(c, info, responsesStreamState, responseID, model, createdAt, streamedThinking.String()); err != nil {
					return err
				}
				thinkingStreamed = true
				clientOutputStarted = true
			}
			chunk := &dto.ChatCompletionsStreamResponse{
				Id:      responseID,
				Object:  "chat.completion.chunk",
				Created: createdAt,
				Model:   model,
				Choices: []dto.ChatCompletionsStreamResponseChoice{{
					Index: 0,
					Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
						Role:    "assistant",
						Content: &chunkText,
					},
				}},
			}
			err := writeCursorStreamChunk(c, info, responsesStreamState, chunk)
			if err == nil {
				clientOutputStarted = true
			}
			return err
		case "thinking":
			var textEvent cursorTextEvent
			if err := common.Unmarshal(event.Data, &textEvent); err != nil {
				return fmt.Errorf("cursor channel: decode thinking event: %w", err)
			}
			if textEvent.Text == "" {
				return nil
			}
			streamedThinking.WriteString(textEvent.Text)
			if !clientStream {
				return nil
			}
			if firstChunk {
				info.FirstResponseTime = time.Now()
				firstChunk = false
			}
			if bufferAssistantForTools {
				return nil
			}
			err := writeCursorThinkingChunk(c, info, responsesStreamState, responseID, model, createdAt, textEvent.Text)
			if err == nil {
				thinkingStreamed = true
				clientOutputStarted = true
			}
			return err
		case "result":
			var result cursorResultEvent
			if err := common.Unmarshal(event.Data, &result); err != nil {
				return fmt.Errorf("cursor channel: decode result event: %w", err)
			}
			resultStatus = result.Status
			finalText = result.Text
		case "status":
			var status cursorResultEvent
			if err := common.Unmarshal(event.Data, &status); err != nil {
				return fmt.Errorf("cursor channel: decode status event: %w", err)
			}
			resultStatus = status.Status
		case "tool_call":
			var toolCall cursorToolCallEvent
			if err := common.Unmarshal(event.Data, &toolCall); err != nil {
				return fmt.Errorf("cursor channel: decode tool call event: %w", err)
			}
			if cursorExternalToolAliasForName(allowedExternalTools, toolCall.Name) != "" {
				lastClientToolCollision = toolCall
			}
		case "error":
			if err := common.Unmarshal(event.Data, &upstreamError); err != nil {
				return fmt.Errorf("cursor channel: decode error event: %w", err)
			}
		case "heartbeat":
			if clientStream {
				return helper.PingData(c)
			}
		}
		return nil
	})

	terminal := cursorRunTerminal(resultStatus)
	finishRun := true
	defer func() {
		if finishRun {
			a.finishCursorRun(c, info, agentID, runID, persistent, terminal)
		}
	}()
	if parseErr != nil {
		return nil, types.NewOpenAIError(parseErr, types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	}
	if upstreamError.Message != "" {
		if isCursorStreamUnavailable(upstreamError.Code) {
			run, err := a.waitCursorRun(c, info, agentID, runID)
			if err != nil {
				return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusBadGateway)
			}
			resultStatus = run.Status
			finalText = run.Result
			terminal = cursorRunTerminal(resultStatus)
			upstreamError = cursorErrorEvent{}
		}
	}
	if upstreamError.Message != "" {
		message := upstreamError.Message
		if upstreamError.Code != "" {
			message = upstreamError.Code + ": " + message
		}
		return nil, types.NewOpenAIError(errors.New(message), types.ErrorCodeBadResponse, http.StatusBadGateway)
	}
	if finalText == "" {
		finalText = streamedText.String()
	}
	if resultStatus != "FINISHED" && agentID != "" && runID != "" {
		run, err := a.waitCursorRun(c, info, agentID, runID)
		if err != nil {
			logger.LogWarn(c, "cursor channel: fetch terminal run detail failed: "+err.Error())
		} else {
			resultStatus = run.Status
			if strings.TrimSpace(run.Result) != "" {
				finalText = run.Result
			}
			terminal = cursorRunTerminal(resultStatus)
		}
	}
	clientToolAlias := cursorExternalToolAliasForName(allowedExternalTools, lastClientToolCollision.Name)
	canRecoverStream := !clientStream || !clientOutputStarted
	shouldRecoverExternalTool := clientToolAlias != "" && canRecoverStream &&
		(resultStatus == "ERROR" || (resultStatus == "FINISHED" && cursorCloudEnvironmentRefusal(finalText)))
	if allowExternalToolRecovery && agentID != "" && shouldRecoverExternalTool {
		logger.LogWarn(c, fmt.Sprintf("cursor channel: intercepted internal tool %q for client tool %q; start one recovery run", lastClientToolCollision.Name, clientToolAlias))
		failedUsage := a.getCursorUsage(c, info, resp, agentID, runID)
		recoveryResponse, err := a.startCursorExternalToolRecoveryRun(
			c,
			info,
			agentID,
			lastClientToolCollision.Name,
			clientToolAlias,
			persistent,
			clientStream,
			resp.Header.Get(cursorSkipRemoteUsageHeader) == "true",
		)
		if err != nil {
			return nil, types.NewOpenAIError(
				fmt.Errorf("cursor channel: recover client external tool after run status %s: %w", resultStatus, err),
				types.ErrorCodeBadResponse,
				http.StatusBadGateway,
			)
		}
		finishRun = false
		return a.doResponse(c, recoveryResponse, info, false, mergeCursorUsage(info, priorUsage, failedUsage))
	}
	if resultStatus != "FINISHED" {
		if resultStatus == "" {
			resultStatus = "missing"
		}
		message := fmt.Sprintf("cursor channel: run ended with status %s", resultStatus)
		if detail := cursorRunErrorDetail(finalText, lastClientToolCollision.Name); detail != "" {
			message += ": " + detail
		}
		return nil, types.NewOpenAIError(errors.New(message), types.ErrorCodeBadResponse, http.StatusBadGateway)
	}
	externalToolCalls, hasExternalToolCalls := parseCursorExternalToolCalls(finalText, allowedExternalTools)
	if clientStream && !bufferAssistantForTools {
		// Normal text has already been streamed, so a differing final result must not
		// append a structured tool call to the same assistant turn.
		externalToolCalls = nil
		hasExternalToolCalls = false
	}
	usage := a.getCursorUsage(c, info, resp, agentID, runID)
	if usage == nil || !service.ValidUsage(usage) {
		usage = service.ResponseText2Usage(c, finalText, model, info.GetEstimatePromptTokens())
	}
	usage = mergeCursorUsage(info, priorUsage, usage)

	if clientStream {
		if !thinkingStreamed && streamedThinking.Len() > 0 {
			if err := writeCursorThinkingChunk(c, info, responsesStreamState, responseID, model, createdAt, streamedThinking.String()); err != nil {
				return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
			}
			thinkingStreamed = true
		}
		if hasExternalToolCalls {
			streamToolCalls := make([]dto.ToolCallResponse, 0, len(externalToolCalls))
			for index, toolCall := range externalToolCalls {
				streamToolCalls = append(streamToolCalls, dto.ToolCallResponse{
					Index: common.GetPointer(index),
					ID:    toolCall.ID,
					Type:  toolCall.Type,
					Function: dto.FunctionResponse{
						Name:      toolCall.Function.Name,
						Arguments: toolCall.Function.Arguments,
					},
				})
			}
			chunk := &dto.ChatCompletionsStreamResponse{
				Id:      responseID,
				Object:  "chat.completion.chunk",
				Created: createdAt,
				Model:   model,
				Choices: []dto.ChatCompletionsStreamResponseChoice{{
					Index: 0,
					Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
						Role:      "assistant",
						ToolCalls: streamToolCalls,
					},
				}},
			}
			if err := writeCursorStreamChunk(c, info, responsesStreamState, chunk); err != nil {
				return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
			}
		} else if (bufferAssistantForTools || streamedText.Len() == 0) && finalText != "" {
			chunk := &dto.ChatCompletionsStreamResponse{
				Id:      responseID,
				Object:  "chat.completion.chunk",
				Created: createdAt,
				Model:   model,
				Choices: []dto.ChatCompletionsStreamResponseChoice{{
					Index: 0,
					Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
						Role:    "assistant",
						Content: &finalText,
					},
				}},
			}
			if err := writeCursorStreamChunk(c, info, responsesStreamState, chunk); err != nil {
				return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
			}
		}
		finishReason := "stop"
		if hasExternalToolCalls {
			finishReason = types.FinishReasonToolCalls
		}
		stopChunk := helper.GenerateStopResponse(responseID, createdAt, model, finishReason)
		if info.RelayFormat == types.RelayFormatClaude || info.RelayFormat == types.RelayFormatOpenAIResponses {
			stopChunk.Usage = usage
		}
		if err := writeCursorStreamChunk(c, info, responsesStreamState, stopChunk); err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		}
		if info.RelayFormat == types.RelayFormatOpenAIResponses {
			if err := finalizeCursorResponseStream(c, info, responsesStreamState); err != nil {
				return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
			}
		}
		if info.RelayFormat == types.RelayFormatOpenAI && info.ShouldIncludeUsage {
			if err := helper.ObjectData(c, helper.GenerateFinalUsageResponse(responseID, createdAt, model, *usage)); err != nil {
				return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
			}
		}
		if info.RelayFormat == types.RelayFormatOpenAI {
			helper.Done(c)
		}
		return usage, nil
	}

	finishReason := "stop"
	message := dto.Message{Role: "assistant", Content: finalText}
	if hasExternalToolCalls {
		finishReason = types.FinishReasonToolCalls
		message.Content = nil
		message.SetToolCalls(externalToolCalls)
	}
	response := &dto.OpenAITextResponse{
		Id:      responseID,
		Object:  "chat.completion",
		Created: createdAt,
		Model:   model,
		Choices: []dto.OpenAITextResponseChoice{{
			Index:        0,
			Message:      message,
			FinishReason: finishReason,
		}},
		Usage: *usage,
	}
	if streamedThinking.Len() > 0 {
		response.Choices[0].Message.ReasoningContent = common.GetPointer(streamedThinking.String())
	}
	if info.RelayFormat == types.RelayFormatClaude || info.RelayFormat == types.RelayFormatOpenAIResponses {
		converted, err := relayconvert.ConvertResponse(c, info, info.RelayFormat, response)
		if err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		}
		if responsesResponse, ok := converted.Value.(*dto.OpenAIResponsesResponse); ok {
			normalizeCursorResponsesOutput(responsesResponse.Output, allowedExternalTools)
		}
		c.JSON(http.StatusOK, converted.Value)
	} else {
		c.JSON(http.StatusOK, response)
	}
	return usage, nil
}

func parseCursorExternalToolCalls(text string, allowedTools map[string]cursorExternalToolSpec) ([]dto.ToolCallRequest, bool) {
	if len(allowedTools) == 0 {
		return nil, false
	}
	payload := strings.TrimSpace(text)
	if strings.HasPrefix(payload, "```json") && strings.HasSuffix(payload, "```") {
		payload = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(payload, "```json"), "```"))
	} else if strings.HasPrefix(payload, "```") && strings.HasSuffix(payload, "```") {
		payload = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(payload, "```"), "```"))
	}

	payloads := []string{payload}
	if markerIndex := strings.Index(payload, `"cursor_external_tool_calls"`); markerIndex >= 0 {
		start := strings.LastIndex(payload[:markerIndex], "{")
		end := strings.LastIndex(payload[markerIndex:], "}")
		if start >= 0 && end >= 0 {
			extracted := strings.TrimSpace(payload[start : markerIndex+end+1])
			if extracted != "" && extracted != payload {
				payloads = append(payloads, extracted)
			}
		}
	}

	var envelope cursorExternalToolEnvelope
	parsed := false
	for _, candidate := range payloads {
		envelope = cursorExternalToolEnvelope{}
		err := common.UnmarshalJsonStr(candidate, &envelope)
		if err != nil {
			repaired := repairCursorExternalToolJSON(candidate)
			if repaired != candidate {
				err = common.UnmarshalJsonStr(repaired, &envelope)
			}
		}
		if err == nil && len(envelope.ToolCalls) > 0 {
			parsed = true
			break
		}
	}
	if !parsed {
		return nil, false
	}
	toolCalls := make([]dto.ToolCallRequest, 0, len(envelope.ToolCalls))
	for _, call := range envelope.ToolCalls {
		name := strings.TrimSpace(call.Name)
		toolSpec, ok := allowedTools[name]
		if !ok {
			return nil, false
		}
		id := strings.TrimSpace(call.ID)
		if id == "" {
			id = "call_" + strings.ReplaceAll(uuid.NewString(), "-", "")
		}
		outputName := cursorQualifiedToolName(toolSpec.Namespace, toolSpec.Name)
		switch toolSpec.Kind {
		case "function":
			arguments := []byte("{}")
			var err error
			switch value := call.Arguments.(type) {
			case nil:
			case string:
				arguments = []byte(strings.TrimSpace(value))
			default:
				arguments, err = common.Marshal(value)
			}
			if err != nil || common.GetJsonType(arguments) != "object" {
				return nil, false
			}
			toolCalls = append(toolCalls, dto.ToolCallRequest{
				ID:   id,
				Type: "function",
				Function: dto.FunctionRequest{
					Name:      outputName,
					Arguments: string(arguments),
				},
			})
		case dto.CustomType:
			input := call.Input
			if input == nil {
				input = call.Arguments
			}
			var inputText string
			switch value := input.(type) {
			case nil:
			case string:
				inputText = value
			default:
				encoded, err := common.Marshal(value)
				if err != nil {
					return nil, false
				}
				inputText = string(encoded)
			}
			toolCalls = append(toolCalls, dto.ToolCallRequest{
				ID:   id,
				Type: dto.CustomType,
				Function: dto.FunctionRequest{
					Name:      outputName,
					Arguments: inputText,
				},
			})
		default:
			return nil, false
		}
	}
	return toolCalls, true
}

func cursorExternalToolAliasForName(allowedTools map[string]cursorExternalToolSpec, name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	for alias, spec := range allowedTools {
		if !strings.HasPrefix(alias, cursorExternalToolAliasPrefix) {
			continue
		}
		if cursorQualifiedToolName(spec.Namespace, spec.Name) == name || spec.Name == name {
			return alias
		}
	}
	return ""
}

func cursorCloudEnvironmentRefusal(text string) bool {
	normalized := strings.ToLower(strings.TrimSpace(text))
	if normalized == "" || !strings.Contains(normalized, "cloud agent") {
		return false
	}
	return strings.Contains(normalized, "no filesystem") ||
		strings.Contains(normalized, "no file system") ||
		strings.Contains(normalized, "only have access to /agent") ||
		(strings.Contains(normalized, "local mac") &&
			(strings.Contains(normalized, "can't complete") || strings.Contains(normalized, "cannot complete")))
}

func cursorRunErrorDetail(text string, internalToolName string) string {
	detail := strings.Join(strings.Fields(text), " ")
	if detail != "" {
		runes := []rune(detail)
		if len(runes) > 1000 {
			detail = string(runes[:1000]) + "…"
		}
		return detail
	}
	internalToolName = strings.TrimSpace(internalToolName)
	if internalToolName == "" {
		return ""
	}
	return fmt.Sprintf("after Cursor internal tool %q", internalToolName)
}

func mergeCursorUsage(info *relaycommon.RelayInfo, prior *dto.Usage, current *dto.Usage) *dto.Usage {
	if prior == nil {
		return current
	}
	if current == nil {
		return prior
	}
	merged := &dto.Usage{
		PromptTokens:         cursorTokenTotal(info, prior.PromptTokens, current.PromptTokens),
		CompletionTokens:     cursorTokenTotal(info, prior.CompletionTokens, current.CompletionTokens),
		PromptCacheHitTokens: cursorTokenTotal(info, prior.PromptCacheHitTokens, current.PromptCacheHitTokens),
		UsageSemantic:        dto.BillingUsageSemanticOpenAI,
		UsageSource:          dto.BillingUsageSourceOAIChat,
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens:         cursorTokenTotal(info, prior.PromptTokensDetails.CachedTokens, current.PromptTokensDetails.CachedTokens),
			CachedCreationTokens: cursorTokenTotal(info, prior.PromptTokensDetails.CachedCreationTokens, current.PromptTokensDetails.CachedCreationTokens),
			CacheWriteTokens:     cursorTokenTotal(info, prior.PromptTokensDetails.CacheWriteTokens, current.PromptTokensDetails.CacheWriteTokens),
			TextTokens:           cursorTokenTotal(info, prior.PromptTokensDetails.TextTokens, current.PromptTokensDetails.TextTokens),
			AudioTokens:          cursorTokenTotal(info, prior.PromptTokensDetails.AudioTokens, current.PromptTokensDetails.AudioTokens),
			ImageTokens:          cursorTokenTotal(info, prior.PromptTokensDetails.ImageTokens, current.PromptTokensDetails.ImageTokens),
		},
		CompletionTokenDetails: dto.OutputTokenDetails{
			TextTokens:      cursorTokenTotal(info, prior.CompletionTokenDetails.TextTokens, current.CompletionTokenDetails.TextTokens),
			AudioTokens:     cursorTokenTotal(info, prior.CompletionTokenDetails.AudioTokens, current.CompletionTokenDetails.AudioTokens),
			ImageTokens:     cursorTokenTotal(info, prior.CompletionTokenDetails.ImageTokens, current.CompletionTokenDetails.ImageTokens),
			ReasoningTokens: cursorTokenTotal(info, prior.CompletionTokenDetails.ReasoningTokens, current.CompletionTokenDetails.ReasoningTokens),
		},
		InputTokens:                 cursorTokenTotal(info, prior.InputTokens, current.InputTokens),
		OutputTokens:                cursorTokenTotal(info, prior.OutputTokens, current.OutputTokens),
		ClaudeCacheCreation5mTokens: cursorTokenTotal(info, prior.ClaudeCacheCreation5mTokens, current.ClaudeCacheCreation5mTokens),
		ClaudeCacheCreation1hTokens: cursorTokenTotal(info, prior.ClaudeCacheCreation1hTokens, current.ClaudeCacheCreation1hTokens),
		Cost:                        current.Cost,
	}
	merged.TotalTokens = cursorTokenTotal(info, merged.PromptTokens, merged.CompletionTokens)
	if prior.InputTokensDetails != nil || current.InputTokensDetails != nil {
		merged.InputTokensDetails = &merged.PromptTokensDetails
	}
	merged.BillingUsage = dto.NewOpenAIChatBillingUsage(merged)
	return merged
}

func repairCursorExternalToolJSON(payload string) string {
	var repaired strings.Builder
	repaired.Grow(len(payload))
	inString := false
	escaped := false
	for index := 0; index < len(payload); index++ {
		character := payload[index]
		if !inString {
			repaired.WriteByte(character)
			if character == '"' {
				inString = true
			}
			continue
		}
		if escaped {
			repaired.WriteByte(character)
			escaped = false
			continue
		}
		if character == '\\' {
			repaired.WriteByte(character)
			escaped = true
			continue
		}
		switch character {
		case '\n':
			repaired.WriteString(`\n`)
			continue
		case '\r':
			repaired.WriteString(`\r`)
			continue
		case '\t':
			repaired.WriteString(`\t`)
			continue
		case '"':
			next := index + 1
			for next < len(payload) && strings.ContainsRune(" \t\r\n", rune(payload[next])) {
				next++
			}
			if next >= len(payload) || strings.ContainsRune(":,}]", rune(payload[next])) {
				repaired.WriteByte(character)
				inString = false
				continue
			}
			repaired.WriteString(`\"`)
			continue
		default:
			repaired.WriteByte(character)
		}
	}
	return repaired.String()
}

func cursorExternalToolStreamCandidate(text string) bool {
	payload := strings.ReplaceAll(strings.TrimLeft(text, " \t\r\n"), "\r\n", "\n")
	marker := `{"cursor_external_tool_calls"`
	for _, prefix := range []string{marker, "```json\n" + marker, "```\n" + marker} {
		if strings.HasPrefix(prefix, payload) || strings.HasPrefix(payload, prefix) {
			return true
		}
	}
	if strings.Contains(payload, marker) {
		return true
	}
	lowerPayload := strings.ToLower(payload)
	for _, correctionPrefix := range []string{
		"i made an error by invoking",
		"i mistakenly invoked",
		"i accidentally invoked",
		"i can't complete this task",
		"i cannot complete this task",
	} {
		if strings.HasPrefix(correctionPrefix, lowerPayload) || strings.HasPrefix(lowerPayload, correctionPrefix) {
			return true
		}
	}
	return false
}

func writeCursorThinkingChunk(
	c *gin.Context,
	info *relaycommon.RelayInfo,
	responsesState *relayconvert.ResponseStreamState,
	responseID string,
	model string,
	createdAt int64,
	thinking string,
) error {
	if thinking == "" {
		return nil
	}
	chunk := &dto.ChatCompletionsStreamResponse{
		Id:      responseID,
		Object:  "chat.completion.chunk",
		Created: createdAt,
		Model:   model,
		Choices: []dto.ChatCompletionsStreamResponseChoice{{
			Index: 0,
			Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
				Role:             "assistant",
				ReasoningContent: &thinking,
			},
		}},
	}
	return writeCursorStreamChunk(c, info, responsesState, chunk)
}

func writeCursorStreamChunk(c *gin.Context, info *relaycommon.RelayInfo, responsesState *relayconvert.ResponseStreamState, chunk *dto.ChatCompletionsStreamResponse) error {
	if info.RelayFormat == types.RelayFormatOpenAI {
		return helper.ObjectData(c, chunk)
	}
	if info.RelayFormat == types.RelayFormatOpenAIResponses {
		if responsesState == nil {
			return errors.New("cursor channel: Responses stream state is required")
		}
		info.IncrSendResponseCount()
		results, err := relayconvert.ConvertStreamResponseChunk(c, info, responsesState, chunk)
		if err != nil {
			return err
		}
		for _, result := range results {
			event, ok := result.Value.(relayconvert.ChatToResponsesStreamEvent)
			if !ok {
				return fmt.Errorf("cursor channel: expected Responses stream event, got %T", result.Value)
			}
			normalizeCursorResponsesStreamEvent(&event.Payload, cursorExternalToolSpecs(c))
			data, err := common.Marshal(event.Payload)
			if err != nil {
				return err
			}
			if err := helper.ResponseChunkData(c, dto.ResponsesStreamResponse{Type: event.Type}, string(data)); err != nil {
				return err
			}
		}
		return nil
	}
	info.IncrSendResponseCount()
	converted, err := relayconvert.ConvertStreamResponse(c, info, types.RelayFormatClaude, chunk)
	if err != nil {
		return err
	}
	responses, ok := converted.Value.([]*dto.ClaudeResponse)
	if !ok {
		return fmt.Errorf("cursor channel: expected Claude stream responses, got %T", converted.Value)
	}
	for _, response := range responses {
		if response == nil {
			continue
		}
		if err := helper.ClaudeData(c, *response); err != nil {
			return err
		}
	}
	return nil
}

func finalizeCursorResponseStream(c *gin.Context, info *relaycommon.RelayInfo, state *relayconvert.ResponseStreamState) error {
	if state == nil {
		return errors.New("cursor channel: Responses stream state is required")
	}
	results, err := relayconvert.FinalizeStreamResponse(c, info, state)
	if err != nil {
		return err
	}
	for _, result := range results {
		event, ok := result.Value.(relayconvert.ChatToResponsesStreamEvent)
		if !ok {
			return fmt.Errorf("cursor channel: expected Responses stream event, got %T", result.Value)
		}
		normalizeCursorResponsesStreamEvent(&event.Payload, cursorExternalToolSpecs(c))
		data, err := common.Marshal(event.Payload)
		if err != nil {
			return err
		}
		if err := helper.ResponseChunkData(c, dto.ResponsesStreamResponse{Type: event.Type}, string(data)); err != nil {
			return err
		}
	}
	return nil
}

func cursorExternalToolSpecs(c *gin.Context) map[string]cursorExternalToolSpec {
	if c == nil {
		return nil
	}
	value, exists := c.Get(cursorExternalToolsContextKey)
	if !exists {
		return nil
	}
	specs, _ := value.(map[string]cursorExternalToolSpec)
	return specs
}

func normalizeCursorResponsesStreamEvent(event *dto.ResponsesStreamResponse, specs map[string]cursorExternalToolSpec) {
	if event == nil || len(specs) == 0 {
		return
	}
	if event.Item != nil {
		if spec, ok := specs[event.Item.Name]; ok && spec.Namespace != "" {
			event.Item.Name = spec.Name
			event.Item.Namespace = spec.Namespace
		}
	}
	if event.Response != nil {
		normalizeCursorResponsesOutput(event.Response.Output, specs)
	}
}

func normalizeCursorResponsesOutput(output []dto.ResponsesOutput, specs map[string]cursorExternalToolSpec) {
	for index := range output {
		spec, ok := specs[output[index].Name]
		if !ok || spec.Namespace == "" {
			continue
		}
		output[index].Name = spec.Name
		output[index].Namespace = spec.Namespace
	}
}

func setCursorAgentResponseHeaders(c *gin.Context, info *relaycommon.RelayInfo, agentID string) {
	if c == nil || info == nil || agentID == "" || strings.TrimSpace(info.ApiKey) == "" {
		return
	}
	userID := c.GetInt("id")
	if userID <= 0 {
		return
	}
	c.Header(constant.CursorAgentIDHeader, agentID)
	c.Header(constant.CursorAgentSignatureHeader, cursorAgentSignature(userID, agentID, info.ChannelId, info.ChannelMultiKeyIndex, info.ApiKey))
	if info.ChannelId > 0 {
		c.Header(constant.CursorAgentChannelIDHeader, strconv.Itoa(info.ChannelId))
	}
	c.Header(constant.CursorAgentKeyIndexHeader, strconv.Itoa(info.ChannelMultiKeyIndex))
}

func writeCursorAgentDeletedResponse(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	const message = "Cursor Agent deleted."
	responseID := helper.GetResponseID(c)
	createdAt := common.GetTimestamp()
	model := info.UpstreamModelName
	usage := service.ResponseText2Usage(c, message, model, info.GetEstimatePromptTokens())
	clientStream, _ := strconv.ParseBool(resp.Header.Get(cursorClientStreamHeader))
	if clientStream {
		helper.SetEventStreamHeaders(c)
		var responsesStreamState *relayconvert.ResponseStreamState
		if info.RelayFormat == types.RelayFormatOpenAIResponses {
			var err error
			responsesStreamState, err = relayconvert.NewResponseStreamState(types.RelayFormatOpenAI, types.RelayFormatOpenAIResponses, relayconvert.ResponseStreamOptions{
				ID:      responseID,
				Model:   model,
				Created: createdAt,
			})
			if err != nil {
				return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
			}
		}
		chunk := &dto.ChatCompletionsStreamResponse{
			Id:      responseID,
			Object:  "chat.completion.chunk",
			Created: createdAt,
			Model:   model,
			Choices: []dto.ChatCompletionsStreamResponseChoice{{
				Index: 0,
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					Role:    "assistant",
					Content: common.GetPointer(message),
				},
			}},
		}
		if err := writeCursorStreamChunk(c, info, responsesStreamState, chunk); err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		}
		stopChunk := helper.GenerateStopResponse(responseID, createdAt, model, "stop")
		if info.RelayFormat == types.RelayFormatClaude || info.RelayFormat == types.RelayFormatOpenAIResponses {
			stopChunk.Usage = usage
		}
		if err := writeCursorStreamChunk(c, info, responsesStreamState, stopChunk); err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		}
		if info.RelayFormat == types.RelayFormatOpenAIResponses {
			if err := finalizeCursorResponseStream(c, info, responsesStreamState); err != nil {
				return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
			}
		}
		if info.RelayFormat == types.RelayFormatOpenAI && info.ShouldIncludeUsage {
			if err := helper.ObjectData(c, helper.GenerateFinalUsageResponse(responseID, createdAt, model, *usage)); err != nil {
				return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
			}
		}
		if info.RelayFormat == types.RelayFormatOpenAI {
			helper.Done(c)
		}
		return usage, nil
	}

	response := &dto.OpenAITextResponse{
		Id:      responseID,
		Object:  "chat.completion",
		Created: createdAt,
		Model:   model,
		Choices: []dto.OpenAITextResponseChoice{{
			Index: 0,
			Message: dto.Message{
				Role:    "assistant",
				Content: message,
			},
			FinishReason: "stop",
		}},
		Usage: *usage,
	}
	if info.RelayFormat == types.RelayFormatClaude || info.RelayFormat == types.RelayFormatOpenAIResponses {
		converted, err := relayconvert.ConvertResponse(c, info, info.RelayFormat, response)
		if err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		}
		c.JSON(http.StatusOK, converted.Value)
	} else {
		c.JSON(http.StatusOK, response)
	}
	return usage, nil
}

func cursorAgentSignature(userID int, agentID string, channelID int, keyIndex int, apiKey string) string {
	payload := cursorAgentSignatureVersion + ":user:" + strconv.Itoa(userID) + ":channel:" + strconv.Itoa(channelID) + ":key:" + strconv.Itoa(keyIndex) + ":" + agentID
	return cursorAgentSignatureVersion + "." + common.GenerateHMACWithKey([]byte("cursor-agent:"+strings.TrimSpace(apiKey)), payload)
}

func (a *Adaptor) getCursorUsage(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response, agentID string, runID string) *dto.Usage {
	if resp.Header.Get(cursorSkipRemoteUsageHeader) == "true" || agentID == "" || runID == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	usagePath := "/v1/agents/" + url.PathEscape(agentID) + "/usage?runId=" + url.QueryEscape(runID)
	usageResponse, err := a.doCursorAPIRequest(ctx, c, info, http.MethodGet, usagePath, nil, false)
	if err != nil {
		logger.LogWarn(c, "cursor channel: fetch usage failed: "+err.Error())
		return nil
	}
	defer service.CloseResponseBodyGracefully(usageResponse)
	if usageResponse.StatusCode != http.StatusOK {
		logger.LogWarn(c, fmt.Sprintf("cursor channel: fetch usage returned status %d", usageResponse.StatusCode))
		return nil
	}
	body, err := io.ReadAll(usageResponse.Body)
	if err != nil {
		logger.LogWarn(c, "cursor channel: read usage failed: "+err.Error())
		return nil
	}
	var parsed cursorUsageResponse
	if err := common.Unmarshal(body, &parsed); err != nil {
		logger.LogWarn(c, "cursor channel: decode usage failed: "+err.Error())
		return nil
	}
	for _, run := range parsed.Runs {
		if run.ID == runID {
			return cursorUsageToOpenAI(run.Usage, info)
		}
	}
	return cursorUsageToOpenAI(parsed.TotalUsage, info)
}

func (a *Adaptor) finishCursorRun(c *gin.Context, info *relaycommon.RelayInfo, agentID string, runID string, persistent bool, terminal bool) {
	if agentID == "" {
		return
	}
	cleanupTimeout := 10 * time.Second
	if info != nil && info.IsChannelTest {
		cleanupTimeout = 2 * time.Second
	}
	if !terminal && runID != "" {
		ctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
		cancelPath := "/v1/agents/" + url.PathEscape(agentID) + "/runs/" + url.PathEscape(runID) + "/cancel"
		cancelResponse, err := a.doCursorAPIRequest(ctx, c, info, http.MethodPost, cancelPath, nil, false)
		cancel()
		if err != nil {
			logger.LogWarn(c, "cursor channel: cancel unfinished run failed: "+err.Error())
		} else {
			service.CloseResponseBodyGracefully(cancelResponse)
		}
	}
	if persistent {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
	defer cancel()
	deletePath := "/v1/agents/" + url.PathEscape(agentID)
	deleteResponse, err := a.doCursorAPIRequest(ctx, c, info, http.MethodDelete, deletePath, nil, false)
	if err != nil {
		logger.LogWarn(c, "cursor channel: delete ephemeral agent failed: "+err.Error())
		return
	}
	service.CloseResponseBodyGracefully(deleteResponse)
}

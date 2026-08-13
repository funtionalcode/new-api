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
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
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
	if resp == nil || resp.Body == nil {
		return nil, types.NewOpenAIError(errors.New("cursor channel: empty upstream response"), types.ErrorCodeBadResponse, http.StatusBadGateway)
	}
	defer service.CloseResponseBodyGracefully(resp)

	agentID := resp.Header.Get(cursorAgentIDInternalHeader)
	runID := resp.Header.Get(cursorRunIDInternalHeader)
	persistent, _ := strconv.ParseBool(resp.Header.Get(cursorPersistentInternalKey))
	clientStream, _ := strconv.ParseBool(resp.Header.Get(cursorClientStreamHeader))
	if persistent && agentID != "" {
		setCursorAgentResponseHeaders(c, agentID)
	}

	responseID := helper.GetResponseID(c)
	createdAt := common.GetTimestamp()
	model := info.UpstreamModelName
	var streamedText strings.Builder
	finalText := ""
	resultStatus := ""
	upstreamError := cursorErrorEvent{}
	firstChunk := true

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
			chunk := &dto.ChatCompletionsStreamResponse{
				Id:      responseID,
				Object:  "chat.completion.chunk",
				Created: createdAt,
				Model:   model,
				Choices: []dto.ChatCompletionsStreamResponseChoice{{
					Index: 0,
					Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
						Role:    "assistant",
						Content: &textEvent.Text,
					},
				}},
			}
			return helper.ObjectData(c, chunk)
		case "result":
			var result cursorResultEvent
			if err := common.Unmarshal(event.Data, &result); err != nil {
				return fmt.Errorf("cursor channel: decode result event: %w", err)
			}
			resultStatus = result.Status
			finalText = result.Text
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

	terminal := resultStatus == "FINISHED" || resultStatus == "ERROR" || resultStatus == "CANCELLED" || resultStatus == "EXPIRED"
	defer a.finishCursorRun(c, info, agentID, runID, persistent, terminal)
	if parseErr != nil {
		return nil, types.NewOpenAIError(parseErr, types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	}
	if upstreamError.Message != "" {
		message := upstreamError.Message
		if upstreamError.Code != "" {
			message = upstreamError.Code + ": " + message
		}
		return nil, types.NewOpenAIError(errors.New(message), types.ErrorCodeBadResponse, http.StatusBadGateway)
	}
	if resultStatus != "FINISHED" {
		if resultStatus == "" {
			resultStatus = "missing"
		}
		return nil, types.NewOpenAIError(fmt.Errorf("cursor channel: run ended with status %s", resultStatus), types.ErrorCodeBadResponse, http.StatusBadGateway)
	}

	if finalText == "" {
		finalText = streamedText.String()
	}
	usage := a.getCursorUsage(c, info, resp, agentID, runID)
	if usage == nil || !service.ValidUsage(usage) {
		usage = service.ResponseText2Usage(c, finalText, model, info.GetEstimatePromptTokens())
	}

	if clientStream {
		if streamedText.Len() == 0 && finalText != "" {
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
			if err := helper.ObjectData(c, chunk); err != nil {
				return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
			}
		}
		if err := helper.ObjectData(c, helper.GenerateStopResponse(responseID, createdAt, model, "stop")); err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		}
		if info.ShouldIncludeUsage {
			if err := helper.ObjectData(c, helper.GenerateFinalUsageResponse(responseID, createdAt, model, *usage)); err != nil {
				return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
			}
		}
		helper.Done(c)
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
				Content: finalText,
			},
			FinishReason: "stop",
		}},
		Usage: *usage,
	}
	c.JSON(http.StatusOK, response)
	return usage, nil
}

func setCursorAgentResponseHeaders(c *gin.Context, agentID string) {
	if c == nil || agentID == "" {
		return
	}
	userID := c.GetInt("id")
	if userID <= 0 {
		return
	}
	c.Header(cursorAgentIDHeader, agentID)
	c.Header(cursorAgentSignatureHeader, cursorAgentSignature(userID, agentID))
}

func cursorAgentSignature(userID int, agentID string) string {
	payload := cursorAgentSignatureVersion + ":user:" + strconv.Itoa(userID) + ":" + agentID
	return cursorAgentSignatureVersion + "." + common.GenerateHMACWithKey([]byte("cursor-agent:"+common.CryptoSecret), payload)
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

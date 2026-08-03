package relay

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	appcommon "github.com/QuantumNous/new-api/common"
	appconstant "github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	responsesWebsocketRequestCreate = "response.create"
	responsesWebsocketRequestAppend = "response.append"
)

type responsesWebsocketTurn struct {
	info            *relaycommon.RelayInfo
	upstreamPayload []byte
	prewarm         bool
}

func ResponsesWebsocketHelper(c *gin.Context, clientWs *websocket.Conn) *types.NewAPIError {
	if clientWs == nil {
		return types.NewError(errors.New("websocket connection is nil"), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}

	var targetWs *websocket.Conn
	var originalModelName string
	var selectedChannelID int
	defer func() {
		if targetWs != nil {
			if err := targetWs.Close(); err != nil {
				logger.LogDebug(c, "responses websocket close upstream error: %s", err.Error())
			}
		}
	}()

	for {
		_, payload, err := readResponsesWebsocketPayload(clientWs)
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseNoStatusReceived) {
				return nil
			}
			return types.NewError(fmt.Errorf("read client websocket failed: %w", err), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
		}

		requestType, explicitModelName, newAPIError := inspectResponsesWebsocketRequest(payload)
		if newAPIError != nil {
			writeResponsesWebsocketAPIError(c, clientWs, newAPIError)
			continue
		}

		firstTurn := originalModelName == ""
		if firstTurn {
			if requestType != responsesWebsocketRequestCreate {
				writeResponsesWebsocketAPIError(c, clientWs, newResponsesWebsocketBadRequest("websocket request received before response.create"))
				continue
			}
			if explicitModelName == "" {
				writeResponsesWebsocketAPIError(c, clientWs, newResponsesWebsocketBadRequest("missing model in response.create request"))
				continue
			}

			selectedChannel, selectErr := middleware.SelectChannelForWebsocketRequest(c, explicitModelName)
			if selectErr != nil {
				writeResponsesWebsocketAPIError(c, clientWs, selectErr)
				continue
			}
			selectedChannelID = selectedChannel.Id
			setResponsesWebsocketUsedChannel(c, selectedChannelID)
			originalModelName = explicitModelName
		} else if explicitModelName != "" && explicitModelName != originalModelName {
			writeResponsesWebsocketAPIError(c, clientWs, newResponsesWebsocketBadRequest("websocket request cannot switch model"))
			continue
		}

		turn, newAPIError := prepareResponsesWebsocketTurn(c, clientWs, payload, originalModelName, firstTurn)
		if newAPIError != nil {
			if firstTurn && targetWs == nil {
				originalModelName = ""
				selectedChannelID = 0
			}
			writeResponsesWebsocketAPIError(c, clientWs, newAPIError)
			continue
		}

		if targetWs == nil {
			target, dialErr := dialResponsesWebsocketUpstream(c, turn.info)
			if dialErr != nil {
				refundResponsesWebsocketTurn(c, turn)
				if firstTurn {
					originalModelName = ""
					selectedChannelID = 0
				}
				writeResponsesWebsocketAPIError(c, clientWs, dialErr)
				continue
			}
			targetWs = target
			logger.LogInfo(c, fmt.Sprintf("responses websocket upstream connected, channelId=%d, model=%s", turn.info.ChannelId, turn.info.OriginModelName))
		}
		turn.info.TargetWs = targetWs

		if err := targetWs.WriteMessage(websocket.TextMessage, turn.upstreamPayload); err != nil {
			refundResponsesWebsocketTurn(c, turn)
			return types.NewError(fmt.Errorf("write upstream websocket failed: %w", err), types.ErrorCodeDoRequestFailed)
		}

		usage, completed, forwardErr := forwardResponsesWebsocketTurn(c, clientWs, targetWs, turn.info)
		if forwardErr != nil {
			refundResponsesWebsocketTurn(c, turn)
			return types.NewError(forwardErr, types.ErrorCodeBadResponse)
		}
		if turn.prewarm {
			continue
		}
		if completed {
			service.PostTextConsumeQuota(c, turn.info, usage, nil)
			if selectedChannelID > 0 {
				service.RecordChannelAffinity(c, selectedChannelID)
			}
			continue
		}
		refundResponsesWebsocketTurn(c, turn)
	}
}

func readResponsesWebsocketPayload(conn *websocket.Conn) (int, []byte, error) {
	for {
		messageType, payload, err := conn.ReadMessage()
		if err != nil {
			return 0, nil, err
		}
		if messageType == websocket.TextMessage || messageType == websocket.BinaryMessage {
			return messageType, payload, nil
		}
	}
}

func inspectResponsesWebsocketRequest(payload []byte) (string, string, *types.NewAPIError) {
	if !gjson.ValidBytes(payload) {
		return "", "", newResponsesWebsocketBadRequest("invalid websocket request JSON")
	}
	requestType := strings.TrimSpace(gjson.GetBytes(payload, "type").String())
	switch requestType {
	case responsesWebsocketRequestCreate, responsesWebsocketRequestAppend:
	default:
		return "", "", newResponsesWebsocketBadRequest(fmt.Sprintf("unsupported websocket request type: %s", requestType))
	}
	return requestType, strings.TrimSpace(gjson.GetBytes(payload, "model").String()), nil
}

func prepareResponsesWebsocketTurn(c *gin.Context, clientWs *websocket.Conn, payload []byte, originalModelName string, firstTurn bool) (*responsesWebsocketTurn, *types.NewAPIError) {
	requestPayload := bytes.Clone(payload)
	if strings.TrimSpace(gjson.GetBytes(requestPayload, "model").String()) == "" {
		var err error
		requestPayload, err = sjson.SetBytes(requestPayload, "model", originalModelName)
		if err != nil {
			return nil, types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
		}
	}
	requestPayload, _ = sjson.SetBytes(requestPayload, "stream", true)

	request := &dto.OpenAIResponsesRequest{}
	if err := appcommon.Unmarshal(requestPayload, request); err != nil {
		return nil, types.NewErrorWithStatusCode(err, types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	prewarm := firstTurn && isResponsesWebsocketPrewarm(requestPayload)
	if _, err := helper.ValidateResponsesRequest(request, !prewarm); err != nil {
		return nil, types.NewErrorWithStatusCode(err, types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	turnRequestID := appcommon.NewRequestId()
	c.Set(appcommon.RequestIdKey, turnRequestID)
	appcommon.SetContextKey(c, appconstant.ContextKeyRequestStartTime, time.Now())
	c.Set("original_model", originalModelName)
	appcommon.SetContextKey(c, appconstant.ContextKeyOriginalModel, originalModelName)

	info, err := relaycommon.GenRelayInfo(c, types.RelayFormatOpenAIResponses, request, clientWs)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeGenRelayInfoFailed, types.ErrOptionWithSkipRetry())
	}
	info.IsStream = true
	info.IsWebsocket = true
	info.RelayMode = relayconstant.RelayModeResponses
	info.ClientWs = clientWs
	info.InitChannelMeta(c)

	if err := helper.ModelMappedHelper(c, info, request); err != nil {
		return nil, types.NewError(err, types.ErrorCodeChannelModelMappedError, types.ErrOptionWithSkipRetry())
	}

	adaptor := GetAdaptor(info.ApiType)
	if adaptor == nil {
		return nil, types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	adaptor.Init(info)

	upstreamPayload, newAPIError := normalizeResponsesWebsocketUpstreamPayload(c, adaptor, info, requestPayload, request)
	if newAPIError != nil {
		return nil, newAPIError
	}

	if !prewarm {
		if newAPIError := preConsumeResponsesWebsocketTurn(c, info, request); newAPIError != nil {
			return nil, newAPIError
		}
	}

	return &responsesWebsocketTurn{
		info:            info,
		upstreamPayload: upstreamPayload,
		prewarm:         prewarm,
	}, nil
}

func normalizeResponsesWebsocketUpstreamPayload(c *gin.Context, adaptor channel.Adaptor, info *relaycommon.RelayInfo, payload []byte, request *dto.OpenAIResponsesRequest) ([]byte, *types.NewAPIError) {
	converted, err := adaptor.ConvertOpenAIResponsesRequest(c, info, *request)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	relaycommon.AppendRequestConversionFromRequest(info, converted)

	convertedRequest, ok := responsesRequestFromConverted(converted)
	if !ok {
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("responses websocket does not support converted upstream request type %T", converted),
			types.ErrorCodeConvertRequestFailed,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}

	jsonData := bytes.Clone(payload)
	var errSet error
	jsonData, errSet = sjson.SetBytes(jsonData, "model", convertedRequest.Model)
	if errSet != nil {
		return nil, types.NewError(errSet, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	jsonData, _ = sjson.SetBytes(jsonData, "stream", true)
	if len(convertedRequest.Instructions) > 0 {
		jsonData, errSet = sjson.SetRawBytes(jsonData, "instructions", convertedRequest.Instructions)
		if errSet != nil {
			return nil, types.NewError(errSet, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}
	}
	if convertedRequest.Reasoning != nil {
		reasoningJSON, err := appcommon.Marshal(convertedRequest.Reasoning)
		if err != nil {
			return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}
		jsonData, errSet = sjson.SetRawBytes(jsonData, "reasoning", reasoningJSON)
		if errSet != nil {
			return nil, types.NewError(errSet, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}
	}
	if len(convertedRequest.Store) > 0 {
		jsonData, errSet = sjson.SetRawBytes(jsonData, "store", convertedRequest.Store)
		if errSet != nil {
			return nil, types.NewError(errSet, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}
	}
	if info.ApiType == appconstant.APITypeCodex {
		if convertedRequest.MaxOutputTokens == nil {
			jsonData, _ = sjson.DeleteBytes(jsonData, "max_output_tokens")
		}
		if convertedRequest.Temperature == nil {
			jsonData, _ = sjson.DeleteBytes(jsonData, "temperature")
		}
	}

	jsonData, err = relaycommon.RemoveDisabledFields(jsonData, info.ChannelOtherSettings, info.ChannelSetting.PassThroughBodyEnabled)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}

	if len(info.ParamOverride) > 0 {
		jsonData, err = relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, info)
		if err != nil {
			return nil, newAPIErrorFromParamOverride(err)
		}
	}

	logger.LogDebug(c, "responses websocket requestBody: %s", jsonData)
	return jsonData, nil
}

func responsesRequestFromConverted(converted any) (dto.OpenAIResponsesRequest, bool) {
	switch request := converted.(type) {
	case dto.OpenAIResponsesRequest:
		return request, true
	case *dto.OpenAIResponsesRequest:
		if request == nil {
			return dto.OpenAIResponsesRequest{}, false
		}
		return *request, true
	default:
		return dto.OpenAIResponsesRequest{}, false
	}
}

func preConsumeResponsesWebsocketTurn(c *gin.Context, info *relaycommon.RelayInfo, request *dto.OpenAIResponsesRequest) *types.NewAPIError {
	meta := request.GetTokenCountMeta()
	if setting.ShouldCheckPromptSensitive() && meta != nil {
		contains, words := service.CheckSensitiveText(meta.CombineText)
		if contains {
			logger.LogWarn(c, fmt.Sprintf("user sensitive words detected: %s", strings.Join(words, ", ")))
			return types.NewError(errors.New("sensitive words detected"), types.ErrorCodeSensitiveWordsDetected, types.ErrOptionWithSkipRetry())
		}
	}

	tokens, err := service.EstimateRequestToken(c, meta, info)
	if err != nil {
		return types.NewError(err, types.ErrorCodeCountTokenFailed)
	}
	info.SetEstimatePromptTokens(tokens)

	priceData, err := helper.ModelPriceHelper(c, info, tokens, meta)
	if err != nil {
		return types.NewError(err, types.ErrorCodeModelPriceError, types.ErrOptionWithStatusCode(http.StatusBadRequest))
	}
	if priceData.FreeModel {
		logger.LogInfo(c, fmt.Sprintf("模型 %s 免费，跳过预扣费", info.OriginModelName))
		return nil
	}
	return service.PreConsumeBilling(c, priceData.QuotaToPreConsume, info)
}

func dialResponsesWebsocketUpstream(c *gin.Context, info *relaycommon.RelayInfo) (*websocket.Conn, *types.NewAPIError) {
	adaptor := GetAdaptor(info.ApiType)
	if adaptor == nil {
		return nil, types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	adaptor.Init(info)
	resp, err := adaptor.DoRequest(c, info, nil)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeDoRequestFailed)
	}
	targetWs, ok := resp.(*websocket.Conn)
	if !ok || targetWs == nil {
		return nil, types.NewError(fmt.Errorf("invalid websocket upstream response: %T", resp), types.ErrorCodeBadResponse, types.ErrOptionWithSkipRetry())
	}
	return targetWs, nil
}

func forwardResponsesWebsocketTurn(c *gin.Context, clientWs *websocket.Conn, targetWs *websocket.Conn, info *relaycommon.RelayInfo) (*dto.Usage, bool, error) {
	usage := &dto.Usage{}
	var responseTextBuilder strings.Builder
	imageCounter := &relaycommon.ImageGenerationCallCounter{}
	imageCommitted := false

	for {
		messageType, message, err := targetWs.ReadMessage()
		if err != nil {
			return usage, false, fmt.Errorf("read upstream websocket failed: %w", err)
		}
		info.SetFirstResponseTime()
		if err := clientWs.WriteMessage(messageType, message); err != nil {
			return usage, false, fmt.Errorf("write client websocket failed: %w", err)
		}

		eventType, terminal, completed := collectResponsesWebsocketUsage(c, info, usage, &responseTextBuilder, message, imageCounter, &imageCommitted)
		if !terminal {
			continue
		}
		finalizeResponsesWebsocketUsage(info, usage, responseTextBuilder.String())
		if !completed {
			logger.LogDebug(c, "responses websocket upstream terminal failure event: %s", eventType)
		}
		return usage, completed, nil
	}
}

func collectResponsesWebsocketUsage(c *gin.Context, info *relaycommon.RelayInfo, usage *dto.Usage, responseTextBuilder *strings.Builder, message []byte, imageCounter *relaycommon.ImageGenerationCallCounter, imageCommitted *bool) (string, bool, bool) {
	trimmed := strings.TrimSpace(string(message))
	if trimmed == "[DONE]" {
		if imageCommitted != nil && !*imageCommitted {
			imageCounter.Commit(info)
			*imageCommitted = true
		}
		return "done", true, true
	}

	var streamResponse dto.ResponsesStreamResponse
	if err := appcommon.Unmarshal(message, &streamResponse); err != nil {
		logger.LogDebug(c, "failed to unmarshal responses websocket frame: %s", err.Error())
		return "", false, false
	}

	if streamResponse.Response != nil {
		applyResponsesUsage(usage, streamResponse.Response.Usage)
	}

	switch streamResponse.Type {
	case "response.output_text.delta":
		responseTextBuilder.WriteString(streamResponse.Delta)
	case dto.ResponsesOutputTypeItemDone:
		if streamResponse.Item != nil && streamResponse.Item.Type == dto.BuildInCallWebSearchCall {
			if info != nil && info.ResponsesUsageInfo != nil && info.ResponsesUsageInfo.BuiltInTools != nil {
				if webSearchTool, exists := info.ResponsesUsageInfo.BuiltInTools[dto.BuildInToolWebSearchPreview]; exists && webSearchTool != nil {
					webSearchTool.CallCount++
				}
			}
		}
		if streamResponse.Item != nil && streamResponse.Item.Type == dto.ResponsesOutputTypeImageGenerationCall && imageCommitted != nil && !*imageCommitted {
			imageCounter.Observe(streamResponse.Item, streamResponse.OutputIndex)
		}
	case "response.completed", "response.done", "response.incomplete":
		if imageCommitted != nil && !*imageCommitted {
			if streamResponse.Response != nil && relaycommon.IsNonBillableResponsesStatus(streamResponse.Response.Status) {
				imageCounter.Reset()
			} else if streamResponse.Response != nil {
				for i := range streamResponse.Response.Output {
					idx := i
					imageCounter.Observe(&streamResponse.Response.Output[i], &idx)
				}
			}
			imageCounter.Commit(info)
			*imageCommitted = true
		}
		return streamResponse.Type, true, true
	case "response.failed", "response.error", "error":
		if imageCommitted != nil && !*imageCommitted {
			imageCounter.Reset()
			imageCounter.Commit(info)
			*imageCommitted = true
		}
		return streamResponse.Type, true, false
	}
	return streamResponse.Type, false, false
}

func applyResponsesUsage(target *dto.Usage, source *dto.Usage) {
	if target == nil || source == nil {
		return
	}
	if source.InputTokens != 0 {
		target.PromptTokens = source.InputTokens
	}
	if source.OutputTokens != 0 {
		target.CompletionTokens = source.OutputTokens
	}
	if source.TotalTokens != 0 {
		target.TotalTokens = source.TotalTokens
	}
	if source.InputTokensDetails != nil {
		target.PromptTokensDetails.CachedTokens = source.InputTokensDetails.CachedTokens
		target.PromptTokensDetails.CacheWriteTokens = source.InputTokensDetails.CacheWriteTokens
	}
	if source.PromptTokens != 0 {
		target.PromptTokens = source.PromptTokens
	}
	if source.CompletionTokens != 0 {
		target.CompletionTokens = source.CompletionTokens
	}
}

func finalizeResponsesWebsocketUsage(info *relaycommon.RelayInfo, usage *dto.Usage, responseText string) {
	if usage == nil {
		return
	}
	if usage.CompletionTokens == 0 && responseText != "" {
		usage.CompletionTokens = service.CountTextToken(responseText, info.UpstreamModelName)
	}
	if usage.PromptTokens == 0 && usage.CompletionTokens != 0 {
		usage.PromptTokens = info.GetEstimatePromptTokens()
	}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}
}

func isResponsesWebsocketPrewarm(payload []byte) bool {
	generate := gjson.GetBytes(payload, "generate")
	return generate.Exists() && !generate.Bool()
}

func refundResponsesWebsocketTurn(c *gin.Context, turn *responsesWebsocketTurn) {
	if turn == nil || turn.prewarm || turn.info == nil || turn.info.Billing == nil {
		return
	}
	turn.info.Billing.Refund(c)
}

func setResponsesWebsocketUsedChannel(c *gin.Context, channelID int) {
	if channelID <= 0 {
		return
	}
	c.Set("use_channel", []string{fmt.Sprintf("%d", channelID)})
}

func writeResponsesWebsocketAPIError(c *gin.Context, clientWs *websocket.Conn, newAPIError *types.NewAPIError) {
	if newAPIError == nil {
		return
	}
	requestID := c.GetString(appcommon.RequestIdKey)
	if requestID == "" {
		requestID = appcommon.NewRequestId()
		c.Set(appcommon.RequestIdKey, requestID)
	}
	newAPIError.SetMessage(appcommon.MessageWithRequestId(newAPIError.Error(), requestID))
	logger.LogError(c, fmt.Sprintf("responses websocket error: %s", appcommon.LocalLogPreview(newAPIError.Error())))
	helper.WssError(c, clientWs, newAPIError.ToOpenAIError())
}

func newResponsesWebsocketBadRequest(message string) *types.NewAPIError {
	return types.NewErrorWithStatusCode(
		errors.New(message),
		types.ErrorCodeInvalidRequest,
		http.StatusBadRequest,
		types.ErrOptionWithSkipRetry(),
	)
}

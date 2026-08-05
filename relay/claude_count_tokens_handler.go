package relay

import (
	"fmt"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/sjson"
)

// ClaudeCountTokensHelper handles Anthropic's token-count endpoint without
// invoking text generation or billing. Native Anthropic and New API channels
// receive the original Claude payload with only the mapped model changed;
// other channel types use the gateway's local token estimator.
func ClaudeCountTokensHelper(c *gin.Context, info *relaycommon.RelayInfo) *types.NewAPIError {
	info.InitChannelMeta(c)

	request, ok := info.Request.(*dto.ClaudeRequest)
	if !ok {
		return types.NewErrorWithStatusCode(fmt.Errorf("invalid request type, expected *dto.ClaudeRequest, got %T", info.Request), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	if err := helper.ModelMappedHelper(c, info, nil); err != nil {
		return types.NewError(err, types.ErrorCodeChannelModelMappedError, types.ErrOptionWithSkipRetry())
	}

	if info.ApiType != constant.APITypeAnthropic && info.ApiType != constant.APITypeNewAPI {
		tokens, err := service.CountRequestToken(c, request.GetTokenCountMeta(), info, info.UpstreamModelName)
		if err != nil {
			return types.NewError(err, types.ErrorCodeCountTokenFailed, types.ErrOptionWithSkipRetry())
		}
		c.JSON(http.StatusOK, gin.H{"input_tokens": tokens})
		return nil
	}

	adaptor := GetAdaptor(info.ApiType)
	if adaptor == nil {
		return types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	adaptor.Init(info)
	info.FinalRequestRelayFormat = types.RelayFormatClaude

	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return types.NewErrorWithStatusCode(err, types.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	rawBody, err := storage.Bytes()
	if err != nil {
		return types.NewErrorWithStatusCode(err, types.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	mappedBody, err := sjson.SetBytes(rawBody, "model", info.UpstreamModelName)
	if err != nil {
		return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}

	requestBody, size, closer, err := relaycommon.NewOutboundJSONBody(mappedBody)
	if err != nil {
		return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	defer closer.Close()
	info.UpstreamRequestBodySize = size

	response, err := adaptor.DoRequest(c, info, requestBody)
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}
	httpResponse, ok := response.(*http.Response)
	if !ok || httpResponse == nil {
		return types.NewOpenAIError(fmt.Errorf("invalid upstream response type: %T", response), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}
	defer service.CloseResponseBodyGracefully(httpResponse)

	statusCodeMapping := c.GetString("status_code_mapping")
	if httpResponse.StatusCode < http.StatusOK || httpResponse.StatusCode >= http.StatusMultipleChoices {
		newAPIError := service.RelayErrorHandler(c.Request.Context(), httpResponse, false)
		service.ResetStatusCode(newAPIError, statusCodeMapping)
		return newAPIError
	}

	responseBody, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		newAPIError := types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
		service.ResetStatusCode(newAPIError, statusCodeMapping)
		return newAPIError
	}
	service.IOCopyBytesGracefully(c, httpResponse, responseBody)
	return nil
}

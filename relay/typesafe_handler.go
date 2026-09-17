package relay

import (
	"bytes"
	"errors"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func TypeSafeHelper(c *gin.Context, info *relaycommon.RelayInfo) *types.NewAPIError {
	info.InitChannelMeta(c)
	if info.ChannelType != constant.ChannelTypeTypeSafe && info.ChannelType != constant.ChannelTypeNewAPI {
		return types.NewError(errors.New("channel does not support /v1/systemone"), types.ErrorCodeInvalidRequest)
	}
	original, ok := info.Request.(*dto.TypeSafeRequest)
	if !ok {
		return types.NewError(errors.New("invalid TypeSafe request"), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	request := *original
	if err := helper.ModelMappedHelper(c, info, &request); err != nil {
		return types.NewError(err, types.ErrorCodeChannelModelMappedError, types.ErrOptionWithSkipRetry())
	}
	data, err := common.Marshal(&request)
	if err != nil {
		return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	if len(info.ParamOverride) > 0 {
		data, err = relaycommon.ApplyParamOverrideWithRelayInfo(data, info)
		if err != nil {
			return newAPIErrorFromParamOverride(err)
		}
		if err = common.Unmarshal(data, &request); err == nil {
			err = helper.ValidateTypeSafeRequest(&request)
		}
		if err != nil {
			return types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
		}
	}
	// new-api 上游使用相同原生协议与响应解析。
	adaptor := GetAdaptor(constant.APITypeTypeSafe)
	adaptor.Init(info)
	response, err := adaptor.DoRequest(c, info, bytes.NewReader(data))
	if err != nil {
		return types.NewError(err, types.ErrorCodeDoRequestFailed)
	}
	httpResponse, ok := response.(*http.Response)
	if !ok || httpResponse == nil {
		return types.NewError(errors.New("invalid TypeSafe HTTP response"), types.ErrorCodeBadResponse)
	}
	if httpResponse.StatusCode != http.StatusOK {
		apiErr := service.RelayErrorHandler(c.Request.Context(), httpResponse, false)
		service.ResetStatusCode(apiErr, c.GetString("status_code_mapping"))
		return apiErr
	}
	usage, apiErr := adaptor.DoResponse(c, httpResponse, info)
	if apiErr != nil {
		return apiErr
	}
	service.PostTextConsumeQuota(c, info, usage.(*dto.Usage), nil)
	return nil
}

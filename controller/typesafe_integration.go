package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type typeSafeResponseWriter struct {
	gin.ResponseWriter
	capture *relaycommon.TypeSafeCapture
}

func (w *typeSafeResponseWriter) Write(data []byte) (int, error) {
	n, err := w.ResponseWriter.Write(data)
	w.capture.Write(data[:n], strings.Contains(w.Header().Get("Content-Type"), "text/event-stream"))
	return n, err
}
func (w *typeSafeResponseWriter) WriteString(text string) (int, error) { return w.Write([]byte(text)) }

func prepareTypeSafeIntegration(c *gin.Context, info *relaycommon.RelayInfo) {
	info.TypeSafeAfter, info.TypeSafeObserve = nil, nil
	info.InitChannelMeta(c)
	raw, exists := info.ParamOverride["_typesafe"]
	if !exists || info.RelayFormat == types.RelayFormatTypeSafe {
		return
	}
	switch info.RelayFormat {
	case types.RelayFormatOpenAI, types.RelayFormatOpenAIResponses, types.RelayFormatClaude, types.RelayFormatGemini:
	default:
		return
	}
	encoded, err := common.Marshal(raw)
	var config dto.TypeSafeIntegration
	if err != nil || common.Unmarshal(encoded, &config) != nil || config.Validate() != nil {
		info.TypeSafeResults = append(info.TypeSafeResults, map[string]any{"status": "error", "reason": "invalid_configuration"})
		return
	}
	if config.Model == "" {
		config.Model = "jev-latest"
	}
	if config.MaxChars == 0 {
		config.MaxChars = 12000
	}
	if config.TimeoutMS == 0 {
		config.TimeoutMS = 5000
	}
	meta := info.Request.GetTokenCountMeta()
	if meta == nil {
		return
	}
	input := &relaycommon.TypeSafeCapture{Limit: config.MaxChars}
	// 使用相同字符边界截取输入，不保留头信息和认证数据。
	inputJSON, _ := common.Marshal(map[string]any{"choices": []any{map[string]any{"text": meta.CombineText}}})
	input.ObserveJSON(inputJSON)
	inputText := input.Text()
	if len(config.Before) > 0 {
		evaluateTypeSafeStage(c, info, config, "before", config.Before, map[string]any{"request": inputText}, input.Truncated)
	}
	if len(config.After) == 0 {
		return
	}
	capture := &relaycommon.TypeSafeCapture{Limit: config.MaxChars}
	if info.IsWebsocket {
		info.TypeSafeObserve = capture.ObserveJSON
	} else {
		c.Writer = &typeSafeResponseWriter{ResponseWriter: c.Writer, capture: capture}
	}
	info.TypeSafeAfter = func() {
		output := capture.Text()
		if output == "" {
			info.TypeSafeResults = append(info.TypeSafeResults, map[string]any{"stage": "after", "status": "skipped", "reason": "no_text_output"})
			return
		}
		evaluateTypeSafeStage(c, info, config, "after", config.After, map[string]any{"request": inputText, "response": output}, input.Truncated || capture.Truncated)
	}
}

func evaluateTypeSafeStage(c *gin.Context, parent *relaycommon.RelayInfo, config dto.TypeSafeIntegration, stage string, questions map[string]dto.TypeSafeQuestion, state map[string]any, truncated bool) {
	result := map[string]any{"stage": stage, "channel_id": config.ChannelID, "model": config.Model, "status": "error", "truncated": truncated}
	parent.TypeSafeResults = append(parent.TypeSafeResults, result)
	if parent.UserSetting.ModelLimitsEnabled && !model.IsModelAllowedByUserLimit(config.Model, model.BuildUserModelLimitMap(model.NormalizeUserModelLimits(parent.UserSetting.ModelLimits))) {
		result["reason"] = "model_not_allowed"
		return
	}
	if common.GetContextKeyBool(c, constant.ContextKeyTokenModelLimitEnabled) {
		value, _ := common.GetContextKey(c, constant.ContextKeyTokenModelLimit)
		limits, _ := value.(map[string]bool)
		if !model.IsModelAllowedByUserLimit(config.Model, limits) {
			result["reason"] = "model_not_allowed"
			return
		}
	}
	target, err := model.GetChannelById(config.ChannelID, true)
	if err != nil || target == nil || target.Type != constant.ChannelTypeTypeSafe || target.Status != common.ChannelStatusEnabled || !target.IsOpenToUser(parent.UserId) {
		result["reason"] = "channel_unavailable_or_forbidden"
		return
	}
	if !common.StringsContains(target.GetModels(), config.Model) || !common.StringsContains(strings.Split(target.Group, ","), parent.UsingGroup) {
		result["reason"] = "model_or_group_not_allowed"
		return
	}
	request := &dto.TypeSafeRequest{Model: config.Model, Questions: questions}
	request.State, err = common.Marshal(state)
	if err != nil || helper.ValidateTypeSafeRequest(request) != nil {
		result["reason"] = "invalid_questions"
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Duration(config.TimeoutMS)*time.Millisecond)
	defer cancel()
	recorder := httptest.NewRecorder()
	child, _ := gin.CreateTestContext(recorder)
	child.Keys = c.Copy().Keys
	child.Request = c.Request.Clone(ctx)
	url := *child.Request.URL
	url.Path, url.RawPath, url.RawQuery = "/v1/systemone", "", ""
	child.Request.URL = &url
	child.Request.Method = http.MethodPost
	childID := common.NewRequestId()
	common.SetContextKey(child, common.RequestIdKey, childID)
	common.SetContextKey(child, constant.ContextKeyRequestStartTime, time.Now())
	child.Set("use_channel", []string{})
	if apiErr := middleware.SetupContextForSelectedChannel(child, target, config.Model); apiErr != nil {
		result["reason"] = "channel_setup_failed"
		return
	}
	childInfo, err := relaycommon.GenRelayInfo(child, types.RelayFormatTypeSafe, request, nil)
	if err != nil {
		result["reason"] = "request_setup_failed"
		return
	}
	childInfo.InitChannelMeta(child)
	childInfo.TypeSafeResults = []map[string]any{{"stage": stage, "parent_request_id": parent.RequestId, "parent_channel_id": parent.ChannelId}}
	meta := request.GetTokenCountMeta()
	tokens, err := service.EstimateRequestToken(child, meta, childInfo)
	if err != nil {
		result["reason"] = "token_estimate_failed"
		return
	}
	childInfo.SetEstimatePromptTokens(tokens)
	price, err := helper.ModelPriceHelper(child, childInfo, tokens, meta)
	if err != nil {
		result["reason"] = "pricing_unavailable"
		return
	}
	if !price.FreeModel {
		childInfo.ForcePreConsume = true
		if apiErr := service.PreConsumeBilling(child, price.QuotaToPreConsume, childInfo); apiErr != nil {
			result["reason"] = "insufficient_quota"
			return
		}
	}
	if apiErr := relay.TypeSafeHelper(child, childInfo); apiErr != nil {
		if childInfo.Billing != nil {
			childInfo.Billing.Refund(child)
		}
		result["reason"] = "evaluation_failed"
		if ctx.Err() != nil {
			result["reason"] = "timeout_or_cancelled"
		}
		return
	}
	var response struct {
		Answers map[string]any `json:"answers"`
		Usage   map[string]any `json:"usage"`
	}
	if common.Unmarshal(recorder.Body.Bytes(), &response) != nil {
		result["reason"] = "invalid_response"
		return
	}
	result["status"], result["request_id"], result["answers"], result["usage"] = "success", childID, response.Answers, response.Usage
}

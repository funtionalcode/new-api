package middleware

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

type ModelRequest struct {
	Model string `json:"model"`
	Group string `json:"group,omitempty"`
}

func Distribute() func(c *gin.Context) {
	return func(c *gin.Context) {
		var channel *model.Channel
		constraints := service.GetChannelConstraints(c)
		constraints.AddFilter(taskdto.ChannelFilter{
			Kind:        taskdto.FilterRequestPath,
			RequestPath: c.Request.URL.Path,
		})
		service.AppendTaskPluginIdentityFilter(c, c.GetString("expected_task_plugin_key"))
		modelRequest, shouldSelectChannel, err := getModelRequest(c)
		if err != nil {
			abortWithOpenAiMessage(c, http.StatusBadRequest, i18n.T(c, i18n.MsgDistributorInvalidRequest, map[string]any{"Error": err.Error()}))
			return
		}
		if modelRequest != nil {
			service.PrepareCursorAgentSession(c, modelRequest.Model)
		}
		// Keep model name for middleware error logs (e.g. token/user model limits).
		if modelRequest != nil && strings.TrimSpace(modelRequest.Model) != "" {
			c.Set("original_model", modelRequest.Model)
			common.SetContextKey(c, constant.ContextKeyOriginalModel, modelRequest.Model)
		}
		requestUserId := c.GetInt("id")
		constraints.AddFilter(taskdto.ChannelFilter{Kind: taskdto.FilterUserAccess, UserId: requestUserId})
		if shouldCheckUserTokenLimit(c, shouldSelectChannel) {
			if requestUserId > 0 {
				limitResult, limitErr := model.CheckUserTokenLimit(requestUserId, time.Now())
				if limitErr != nil {
					common.SysLog(fmt.Sprintf("check user token limit failed, user_id=%d: %s", requestUserId, limitErr.Error()))
					abortWithOpenAiMessage(c, http.StatusInternalServerError, i18n.T(c, i18n.MsgDatabaseError))
					return
				}
				if limitResult.Exceeded {
					abortWithOpenAiMessage(c, http.StatusForbidden, limitResult.Message(), types.ErrorCodeInsufficientUserQuota)
					return
				}
			}
		}
		if pin, found, overridden := constraints.ResolvedPin(); found {
			for _, lost := range overridden {
				logger.LogWarn(c, fmt.Sprintf(
					"channel pin overridden: winning_source=%s winning_channel_id=%d overridden_source=%s overridden_channel_id=%d",
					pin.Source, pin.ChannelId, lost.Source, lost.ChannelId,
				))
			}
			channel, err = model.CacheGetChannel(pin.ChannelId)
			if err != nil {
				if pin.Source == taskdto.PinSourceOriginTask {
					abortWithOpenAiMessage(c, http.StatusBadRequest, "origin_task_channel_disabled", types.ErrorCode("origin_task_channel_disabled"))
				} else {
					abortWithOpenAiMessage(c, http.StatusBadRequest, i18n.T(c, i18n.MsgDistributorInvalidChannelId))
				}
				return
			}
			if channel.Status != common.ChannelStatusEnabled {
				if pin.Source == taskdto.PinSourceOriginTask {
					abortWithOpenAiMessage(c, http.StatusBadRequest, "origin_task_channel_disabled", types.ErrorCode("origin_task_channel_disabled"))
				} else {
					abortWithOpenAiMessage(c, http.StatusForbidden, i18n.T(c, i18n.MsgDistributorChannelDisabled))
				}
				return
			}
			if ok, kind := model.ChannelSatisfiesFilters(channel, modelRequest.Model, constraints.Filters); !ok {
				if kind == taskdto.FilterTaskPluginIdentity {
					logTaskPluginChannelDecision(c, channel, modelRequest.Model, "channel_rejected", "identity_mismatch")
				}
				abortWithOpenAiMessage(c, http.StatusBadRequest, i18n.T(c, i18n.MsgDistributorNoAvailableChannel, map[string]any{"Group": common.GetContextKeyString(c, constant.ContextKeyUsingGroup), "Model": modelRequest.Model}), types.ErrorCode(kind))
				return
			}
			if !channel.IsOpenToUser(requestUserId) {
				abortWithOpenAiMessage(c, http.StatusForbidden, "该渠道未开放给当前用户")
				return
			}
			if shouldSelectChannel && !channelSupportsRequestPath(channel, c.Request.URL.Path, modelRequest.Model) {
				message := i18n.T(c, i18n.MsgDistributorInvalidRequest, map[string]any{"Error": fmt.Sprintf("channel does not support request path %s", c.Request.URL.Path)})
				abortWithOpenAiMessage(c, http.StatusBadRequest, message, types.ErrorCodeInvalidRequest)
				return
			}
		} else {
			// Select a channel for the user
			// check token model mapping
			modelLimitEnable := common.GetContextKeyBool(c, constant.ContextKeyTokenModelLimitEnabled)
			if modelLimitEnable && (shouldSelectChannel || strings.TrimSpace(modelRequest.Model) != "") {
				s, ok := common.GetContextKey(c, constant.ContextKeyTokenModelLimit)
				if !ok {
					// token model limit is empty, all models are not allowed
					abortWithOpenAiMessage(c, http.StatusForbidden, i18n.T(c, i18n.MsgDistributorTokenNoModelAccess))
					return
				}
				var tokenModelLimit map[string]bool
				tokenModelLimit, ok = s.(map[string]bool)
				if !ok {
					tokenModelLimit = map[string]bool{}
				}
				if !model.IsModelAllowedByUserLimit(modelRequest.Model, tokenModelLimit) {
					abortWithOpenAiMessage(c, http.StatusForbidden, i18n.T(c, i18n.MsgDistributorTokenModelForbidden, map[string]any{"Model": modelRequest.Model}))
					return
				}
			}

			if shouldSelectChannel {
				if modelRequest.Model == "" {
					abortWithOpenAiMessage(c, http.StatusBadRequest, i18n.T(c, i18n.MsgDistributorModelNameRequired))
					return
				}
				if !isModelAllowedByUser(c, modelRequest.Model) {
					abortWithOpenAiMessage(c, http.StatusForbidden, i18n.T(c, i18n.MsgDistributorUserModelForbidden, map[string]any{"Model": modelRequest.Model}))
					return
				}
				var selectGroup string
				usingGroup := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
				if strings.HasPrefix(c.Request.URL.Path, "/pg/") && modelRequest.Group != "" {
					if !service.GroupInUserUsableGroups(usingGroup, modelRequest.Group) && modelRequest.Group != usingGroup {
						abortWithOpenAiMessage(c, http.StatusForbidden, i18n.T(c, i18n.MsgDistributorGroupAccessDenied))
						return
					}
					usingGroup = modelRequest.Group
					common.SetContextKey(c, constant.ContextKeyUsingGroup, usingGroup)
				}
				if isCursorPersistentRequestPath(c.Request.URL.Path) && strings.TrimSpace(c.GetHeader(constant.CursorAgentIDHeader)) != "" {
					persistentChannelID, channelErr := strconv.Atoi(strings.TrimSpace(c.GetHeader(constant.CursorAgentChannelIDHeader)))
					persistentKeyIndex, keyErr := strconv.Atoi(strings.TrimSpace(c.GetHeader(constant.CursorAgentKeyIndexHeader)))
					if channelErr != nil || persistentChannelID <= 0 || keyErr != nil || persistentKeyIndex < 0 {
						abortWithOpenAiMessage(c, http.StatusBadRequest, i18n.T(c, i18n.MsgDistributorInvalidChannelId))
						return
					}
					preferred, preferredErr := model.CacheGetChannel(persistentChannelID)
					if preferredErr != nil || preferred == nil || preferred.Type != constant.ChannelTypeCursor {
						abortWithOpenAiMessage(c, http.StatusBadRequest, i18n.T(c, i18n.MsgDistributorInvalidChannelId))
						return
					}
					if preferred.Status != common.ChannelStatusEnabled {
						abortWithOpenAiMessage(c, http.StatusForbidden, i18n.T(c, i18n.MsgDistributorChannelDisabled))
						return
					}
					if !preferred.IsOpenToUser(requestUserId) || !channelSupportsRequestPath(preferred, c.Request.URL.Path, modelRequest.Model) {
						abortWithOpenAiMessage(c, http.StatusForbidden, i18n.T(c, i18n.MsgDistributorUserModelForbidden, map[string]any{"Model": modelRequest.Model}))
						return
					}
					if usingGroup == "auto" {
						userGroup := common.GetContextKeyString(c, constant.ContextKeyUserGroup)
						for _, candidateGroup := range service.GetRequestAutoGroups(c, userGroup) {
							if model.IsChannelEnabledForGroupModel(candidateGroup, modelRequest.Model, preferred.Id) {
								selectGroup = candidateGroup
								common.SetContextKey(c, constant.ContextKeyAutoGroup, candidateGroup)
								break
							}
						}
					} else if model.IsChannelEnabledForGroupModel(usingGroup, modelRequest.Model, preferred.Id) {
						selectGroup = usingGroup
					}
					if selectGroup == "" {
						abortWithOpenAiMessage(c, http.StatusForbidden, i18n.T(c, i18n.MsgDistributorUserModelForbidden, map[string]any{"Model": modelRequest.Model}))
						return
					}
					channel = preferred
					c.Set("specific_channel_id", strconv.Itoa(preferred.Id))
					common.SetContextKey(c, constant.ContextKeyChannelKeyIndexOverride, persistentKeyIndex)
				}

				if channel == nil {
					if preferredChannelID, found := service.GetPreferredChannelByAffinity(c, modelRequest.Model, usingGroup); found {
						affinityUsable := false
						preferred, err := model.CacheGetChannel(preferredChannelID)
						if err == nil && preferred != nil {
							if preferred.Status != common.ChannelStatusEnabled {
								if service.ShouldSkipRetryAfterChannelAffinityFailure(c) {
									abortWithOpenAiMessage(c, http.StatusForbidden, i18n.T(c, i18n.MsgDistributorAffinityChannelDisabled))
									return
								}
							} else if ok, kind := model.ChannelSatisfiesFilters(preferred, modelRequest.Model, constraints.Filters); !ok {
								logger.LogDebug(c, "affinity channel %d failed filter %s, ignore it", preferred.Id, kind)
							} else if !channelSupportsRequestPath(preferred, c.Request.URL.Path, modelRequest.Model) {
								logger.LogDebug(c, "affinity channel %d does not support request path %s, ignore it", preferred.Id, c.Request.URL.Path)
							} else if !preferred.IsOpenToUser(requestUserId) {
								logger.LogDebug(c, "affinity channel %d is not open to user %d, ignore it", preferred.Id, requestUserId)
							} else if usingGroup == "auto" {
								userGroup := common.GetContextKeyString(c, constant.ContextKeyUserGroup)
								autoGroups := service.GetRequestAutoGroups(c, userGroup)
								for _, g := range autoGroups {
									if model.IsChannelEnabledForGroupModel(g, modelRequest.Model, preferred.Id) {
										selectGroup = g
										common.SetContextKey(c, constant.ContextKeyAutoGroup, g)
										channel = preferred
										affinityUsable = true
										service.MarkChannelAffinityUsed(c, g, preferred.Id)
										break
									}
								}
							} else if model.IsChannelEnabledForGroupModel(usingGroup, modelRequest.Model, preferred.Id) {
								channel = preferred
								selectGroup = usingGroup
								affinityUsable = true
								service.MarkChannelAffinityUsed(c, usingGroup, preferred.Id)
							}
						}
						if !affinityUsable && !service.ShouldKeepChannelAffinityOnChannelDisabled() {
							service.ClearCurrentChannelAffinityCache(c)
						}
					}
				}

				if channel == nil {
					channel, selectGroup, err = service.CacheGetRandomSatisfiedChannel(&service.RetryParam{
						Ctx:         c,
						ModelName:   modelRequest.Model,
						TokenGroup:  usingGroup,
						RequestPath: c.Request.URL.Path,
						Retry:       common.GetPointer(0),
					})
					if err != nil {
						showGroup := usingGroup
						if usingGroup == "auto" {
							showGroup = fmt.Sprintf("auto(%s)", selectGroup)
						}
						message := i18n.T(c, i18n.MsgDistributorGetChannelFailed, map[string]any{"Group": showGroup, "Model": modelRequest.Model, "Error": err.Error()})
						// 如果错误，但是渠道不为空，说明是数据库一致性问题
						//if channel != nil {
						//	common.SysError(fmt.Sprintf("渠道不存在：%d", channel.Id))
						//	message = "数据库一致性已被破坏，请联系管理员"
						//}
						abortWithOpenAiMessage(c, http.StatusServiceUnavailable, message, types.ErrorCodeModelNotFound)
						return
					}
					if channel == nil {
						abortWithOpenAiMessage(c, http.StatusServiceUnavailable, i18n.T(c, i18n.MsgDistributorNoAvailableChannel, map[string]any{"Group": usingGroup, "Model": modelRequest.Model}), types.ErrorCodeModelNotFound)
						return
					}
				}
			}
		}
		if channel != nil {
			if ok, kind := model.ChannelSatisfiesFilters(channel, modelRequest.Model, constraints.Filters); !ok {
				if kind == taskdto.FilterTaskPluginIdentity {
					logTaskPluginChannelDecision(c, channel, modelRequest.Model, "channel_rejected", "identity_mismatch")
				}
				abortWithOpenAiMessage(c, http.StatusServiceUnavailable, i18n.T(c, i18n.MsgDistributorNoAvailableChannel, map[string]any{"Group": common.GetContextKeyString(c, constant.ContextKeyUsingGroup), "Model": modelRequest.Model}), types.ErrorCodeModelNotFound)
				return
			}
		}
		common.SetContextKey(c, constant.ContextKeyRequestStartTime, time.Now())
		if isCursorPersistentRequestPath(c.Request.URL.Path) && strings.TrimSpace(c.GetHeader(constant.CursorAgentIDHeader)) != "" {
			savedChannelID := strings.TrimSpace(c.GetHeader(constant.CursorAgentChannelIDHeader))
			savedKeyIndex := strings.TrimSpace(c.GetHeader(constant.CursorAgentKeyIndexHeader))
			persistentChannelID, channelErr := strconv.Atoi(savedChannelID)
			persistentKeyIndex, keyErr := strconv.Atoi(savedKeyIndex)
			if savedChannelID == "" || savedKeyIndex == "" || channelErr != nil || keyErr != nil || persistentKeyIndex < 0 || channel == nil || channel.Type != constant.ChannelTypeCursor || channel.Id != persistentChannelID {
				abortWithOpenAiMessage(c, http.StatusBadRequest, i18n.T(c, i18n.MsgDistributorInvalidChannelId))
				return
			}
			common.SetContextKey(c, constant.ContextKeyChannelKeyIndexOverride, persistentKeyIndex)
		}
		if newAPIError := SetupContextForSelectedChannel(c, channel, modelRequest.Model); newAPIError != nil {
			if strings.TrimSpace(c.GetHeader(constant.CursorAgentIDHeader)) != "" {
				abortWithOpenAiMessage(c, newAPIError.StatusCode, newAPIError.Error(), newAPIError.GetErrorCode())
				return
			}
		}
		c.Next()
		if channel != nil && c.Writer != nil && c.Writer.Status() < http.StatusBadRequest {
			service.RecordChannelAffinity(c, channel.Id)
		}
	}
}

func shouldCheckUserTokenLimit(c *gin.Context, shouldSelectChannel bool) bool {
	if shouldSelectChannel {
		return true
	}
	relayModeValue, ok := c.Get("relay_mode")
	if !ok {
		return false
	}
	relayMode, ok := relayModeValue.(int)
	if !ok {
		return false
	}
	return relayMode == relayconstant.RelayModeVideoSubmit
}

func SelectChannelForWebsocketRequest(c *gin.Context, modelName string) (*model.Channel, *types.NewAPIError) {
	modelRequest := &ModelRequest{Model: strings.TrimSpace(modelName)}
	if modelRequest.Model == "" {
		return nil, types.NewErrorWithStatusCode(
			errors.New(i18n.T(c, i18n.MsgDistributorModelNameRequired)),
			types.ErrorCodeInvalidRequest,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}

	c.Set("original_model", modelRequest.Model)
	common.SetContextKey(c, constant.ContextKeyOriginalModel, modelRequest.Model)

	requestUserId := c.GetInt("id")
	constraints := service.GetChannelConstraints(c)
	constraints.AddFilter(taskdto.ChannelFilter{Kind: taskdto.FilterRequestPath, RequestPath: c.Request.URL.Path})
	constraints.AddFilter(taskdto.ChannelFilter{Kind: taskdto.FilterUserAccess, UserId: requestUserId})
	if requestUserId > 0 {
		limitResult, limitErr := model.CheckUserTokenLimit(requestUserId, time.Now())
		if limitErr != nil {
			common.SysLog(fmt.Sprintf("check user token limit failed, user_id=%d: %s", requestUserId, limitErr.Error()))
			return nil, types.NewErrorWithStatusCode(
				errors.New(i18n.T(c, i18n.MsgDatabaseError)),
				types.ErrorCodeQueryDataError,
				http.StatusInternalServerError,
				types.ErrOptionWithSkipRetry(),
			)
		}
		if limitResult.Exceeded {
			return nil, types.NewErrorWithStatusCode(
				errors.New(limitResult.Message()),
				types.ErrorCodeInsufficientUserQuota,
				http.StatusForbidden,
				types.ErrOptionWithSkipRetry(),
			)
		}
	}

	var channel *model.Channel
	if pin, found, _ := constraints.ResolvedPin(); found {
		var err error
		channel, err = model.CacheGetChannel(pin.ChannelId)
		if err != nil {
			return nil, types.NewErrorWithStatusCode(
				errors.New(i18n.T(c, i18n.MsgDistributorInvalidChannelId)),
				types.ErrorCodeGetChannelFailed,
				http.StatusBadRequest,
				types.ErrOptionWithSkipRetry(),
			)
		}
		if channel.Status != common.ChannelStatusEnabled {
			return nil, types.NewErrorWithStatusCode(
				errors.New(i18n.T(c, i18n.MsgDistributorChannelDisabled)),
				types.ErrorCodeGetChannelFailed,
				http.StatusForbidden,
				types.ErrOptionWithSkipRetry(),
			)
		}
		if !channel.IsOpenToUser(requestUserId) {
			return nil, types.NewErrorWithStatusCode(
				errors.New("该渠道未开放给当前用户"),
				types.ErrorCodeAccessDenied,
				http.StatusForbidden,
				types.ErrOptionWithSkipRetry(),
			)
		}
		if ok, kind := model.ChannelSatisfiesFilters(channel, modelRequest.Model, constraints.Filters); !ok {
			return nil, types.NewErrorWithStatusCode(
				fmt.Errorf("channel does not satisfy %s constraint", kind),
				types.ErrorCodeAccessDenied,
				http.StatusForbidden,
				types.ErrOptionWithSkipRetry(),
			)
		}
	} else {
		modelLimitEnable := common.GetContextKeyBool(c, constant.ContextKeyTokenModelLimitEnabled)
		if modelLimitEnable {
			s, ok := common.GetContextKey(c, constant.ContextKeyTokenModelLimit)
			if !ok {
				return nil, types.NewErrorWithStatusCode(
					errors.New(i18n.T(c, i18n.MsgDistributorTokenNoModelAccess)),
					types.ErrorCodeAccessDenied,
					http.StatusForbidden,
					types.ErrOptionWithSkipRetry(),
				)
			}
			tokenModelLimit, ok := s.(map[string]bool)
			if !ok {
				tokenModelLimit = map[string]bool{}
			}
			if !model.IsModelAllowedByUserLimit(modelRequest.Model, tokenModelLimit) {
				return nil, types.NewErrorWithStatusCode(
					errors.New(i18n.T(c, i18n.MsgDistributorTokenModelForbidden, map[string]any{"Model": modelRequest.Model})),
					types.ErrorCodeAccessDenied,
					http.StatusForbidden,
					types.ErrOptionWithSkipRetry(),
				)
			}
		}

		if !isModelAllowedByUser(c, modelRequest.Model) {
			return nil, types.NewErrorWithStatusCode(
				errors.New(i18n.T(c, i18n.MsgDistributorUserModelForbidden, map[string]any{"Model": modelRequest.Model})),
				types.ErrorCodeAccessDenied,
				http.StatusForbidden,
				types.ErrOptionWithSkipRetry(),
			)
		}

		var selectGroup string
		usingGroup := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
		if preferredChannelID, found := service.GetPreferredChannelByAffinity(c, modelRequest.Model, usingGroup); found {
			affinityUsable := false
			preferred, err := model.CacheGetChannel(preferredChannelID)
			if err == nil && preferred != nil {
				if preferred.Status != common.ChannelStatusEnabled {
					if service.ShouldSkipRetryAfterChannelAffinityFailure(c) {
						return nil, types.NewErrorWithStatusCode(
							errors.New(i18n.T(c, i18n.MsgDistributorAffinityChannelDisabled)),
							types.ErrorCodeGetChannelFailed,
							http.StatusForbidden,
							types.ErrOptionWithSkipRetry(),
						)
					}
				} else if !channelSupportsRequestPath(preferred, c.Request.URL.Path, modelRequest.Model) {
					logger.LogDebug(c, "affinity channel %d does not support request path %s, ignore it", preferred.Id, c.Request.URL.Path)
				} else if !preferred.IsOpenToUser(requestUserId) {
					logger.LogDebug(c, "affinity channel %d is not open to user %d, ignore it", preferred.Id, requestUserId)
				} else if usingGroup == "auto" {
					userGroup := common.GetContextKeyString(c, constant.ContextKeyUserGroup)
					autoGroups := service.GetUserAutoGroup(userGroup)
					for _, g := range autoGroups {
						if model.IsChannelEnabledForGroupModel(g, modelRequest.Model, preferred.Id) {
							selectGroup = g
							common.SetContextKey(c, constant.ContextKeyAutoGroup, g)
							channel = preferred
							affinityUsable = true
							service.MarkChannelAffinityUsed(c, g, preferred.Id)
							break
						}
					}
				} else if model.IsChannelEnabledForGroupModel(usingGroup, modelRequest.Model, preferred.Id) {
					channel = preferred
					selectGroup = usingGroup
					affinityUsable = true
					service.MarkChannelAffinityUsed(c, usingGroup, preferred.Id)
				}
			}
			if !affinityUsable && !service.ShouldKeepChannelAffinityOnChannelDisabled() {
				service.ClearCurrentChannelAffinityCache(c)
			}
		}

		if channel == nil {
			var err error
			channel, selectGroup, err = service.CacheGetRandomSatisfiedChannel(&service.RetryParam{
				Ctx:         c,
				ModelName:   modelRequest.Model,
				TokenGroup:  usingGroup,
				RequestPath: c.Request.URL.Path,
				Retry:       common.GetPointer(0),
			})
			if err != nil {
				showGroup := usingGroup
				if usingGroup == "auto" {
					showGroup = fmt.Sprintf("auto(%s)", selectGroup)
				}
				return nil, types.NewErrorWithStatusCode(
					fmt.Errorf("%s", i18n.T(c, i18n.MsgDistributorGetChannelFailed, map[string]any{"Group": showGroup, "Model": modelRequest.Model, "Error": err.Error()})),
					types.ErrorCodeModelNotFound,
					http.StatusServiceUnavailable,
					types.ErrOptionWithSkipRetry(),
				)
			}
			if channel == nil {
				return nil, types.NewErrorWithStatusCode(
					errors.New(i18n.T(c, i18n.MsgDistributorNoAvailableChannel, map[string]any{"Group": usingGroup, "Model": modelRequest.Model})),
					types.ErrorCodeModelNotFound,
					http.StatusServiceUnavailable,
					types.ErrOptionWithSkipRetry(),
				)
			}
		}
	}

	common.SetContextKey(c, constant.ContextKeyRequestStartTime, time.Now())
	if newAPIError := SetupContextForSelectedChannel(c, channel, modelRequest.Model); newAPIError != nil {
		return nil, newAPIError
	}
	return channel, nil
}

// channelSupportsRequestPath reports whether a channel can serve the request path.
// Advanced Custom channels use their configured routes. Cursor exposes text
// chat through both the OpenAI Chat Completions and Anthropic Messages surfaces.
func channelSupportsRequestPath(channel *model.Channel, requestPath string, requestModel string) bool {
	return channel.SupportsRequestPath(requestPath, requestModel)
}

func isCursorPersistentRequestPath(requestPath string) bool {
	switch normalizePlaygroundRequestPath(requestPath) {
	case "/v1/chat/completions", "/v1/messages", "/v1/responses":
		return true
	default:
		return false
	}
}

func normalizePlaygroundRequestPath(requestPath string) string {
	switch {
	case strings.HasPrefix(requestPath, "/pg/chat/completions"):
		return "/v1/chat/completions"
	case strings.HasPrefix(requestPath, "/pg/images/generations"):
		return "/v1/images/generations"
	case strings.HasPrefix(requestPath, "/pg/audio/transcriptions"):
		return "/v1/audio/transcriptions"
	default:
		return requestPath
	}
}

func channelMatchesExpectedTaskPlugin(c *gin.Context, channel *model.Channel, expected string) bool {
	if channel == nil {
		return false
	}
	if c != nil {
		if _, matched := pinnedEndpointCandidateForChannel(c, channel, expected); matched {
			return true
		}
	}
	if channel.Type == constant.ChannelTypeTaskPlugin {
		return expected != "" && channel.GetSetting().TaskPluginKey == expected
	}
	if expected == "" {
		return true
	}

	if c == nil {
		return false
	}
	value, exists := c.Get(jsplugin.ContextKeyPinnedPlugin)
	pinned, ok := value.(jsplugin.PinnedPlugin)
	if !exists || !ok || pinned.Generation == nil || pinned.Plugin == nil || pinned.Plugin.Meta.Key != expected {
		return false
	}
	plugin, ok := pinned.Generation.GetByChannelType(channel.Type)
	return ok && plugin == pinned.Plugin
}

func pinnedEndpointCandidateForChannel(c *gin.Context, channel *model.Channel, expected string) (jsplugin.ProtocolBinding, bool) {
	if c == nil || channel == nil || expected == "" {
		return jsplugin.ProtocolBinding{}, false
	}
	value, exists := c.Get(jsplugin.ContextKeyPinnedEndpoint)
	pinned, ok := value.(jsplugin.PinnedEndpoint)
	if !exists || !ok || pinned.Generation == nil || pinned.Plugin == nil {
		return jsplugin.ProtocolBinding{}, false
	}
	candidates := pinned.Candidates
	if len(candidates) == 0 {
		candidates = []jsplugin.ProtocolBinding{{Plugin: pinned.Plugin, Protocol: pinned.Protocol, Operation: pinned.Operation, Model: pinned.Model}}
	}
	expectedOwned := false
	selected := jsplugin.ProtocolBinding{}
	for _, candidate := range candidates {
		if candidate.Plugin == nil {
			continue
		}
		if candidate.Plugin.Meta.Key == expected {
			expectedOwned = true
		}
		if channel.Type == constant.ChannelTypeTaskPlugin {
			if channel.GetSetting().TaskPluginKey == candidate.Plugin.Meta.Key {
				selected = candidate
			}
			continue
		}
		plugin, indexed := pinned.Generation.GetByChannelType(channel.Type)
		if indexed && plugin == candidate.Plugin {
			selected = candidate
		}
	}
	return selected, expectedOwned && selected.Plugin != nil
}

// getModelFromRequest 从请求中读取模型信息
// 根据 Content-Type 自动处理：
// - application/json
// - application/x-www-form-urlencoded
// - multipart/form-data
func getModelFromRequest(c *gin.Context) (*ModelRequest, error) {
	if cached, exists := c.Get(contextKeyTaskPluginEndpointModel); exists {
		if modelRequest, ok := cached.(ModelRequest); ok {
			cachedRequest := modelRequest
			return &cachedRequest, nil
		}
	}
	if strings.HasPrefix(c.Request.Header.Get("Content-Type"), "application/json") {
		modelRequest, err := getModelFromJSONBody(c)
		if err != nil {
			return nil, errors.New(i18n.T(c, i18n.MsgDistributorInvalidRequest, map[string]any{"Error": err.Error()}))
		}
		return modelRequest, nil
	}

	var modelRequest ModelRequest
	err := common.UnmarshalBodyReusable(c, &modelRequest)
	if err != nil {
		return nil, errors.New(i18n.T(c, i18n.MsgDistributorInvalidRequest, map[string]any{"Error": err.Error()}))
	}
	return &modelRequest, nil
}

func getModelFromJSONBody(c *gin.Context) (*ModelRequest, error) {
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return nil, err
	}
	requestBody, err := storage.Bytes()
	if err != nil {
		return nil, err
	}
	if !gjson.ValidBytes(requestBody) {
		return nil, errors.New("invalid JSON request body")
	}
	if countTopLevelJSONKey(requestBody, "model") > 1 {
		return nil, errors.New("model must be provided once")
	}

	values := gjson.GetManyBytes(requestBody, "model", "group")
	model, err := getJSONStringValue(values[0], "model")
	if err != nil {
		return nil, err
	}
	group, err := getJSONStringValue(values[1], "group")
	if err != nil {
		return nil, err
	}

	if _, seekErr := storage.Seek(0, io.SeekStart); seekErr != nil {
		return nil, seekErr
	}
	c.Request.Body = io.NopCloser(storage)

	return &ModelRequest{
		Model: model,
		Group: group,
	}, nil
}

func countTopLevelJSONKey(data []byte, target string) int {
	depth := 0
	inString := false
	escaped := false
	stringStart := 0
	expectingKey := false
	count := 0
	for index, current := range data {
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if current == '\\' {
				escaped = true
				continue
			}
			if current != '"' {
				continue
			}
			inString = false
			if depth == 1 && expectingKey {
				key := string(data[stringStart:index])
				var decodedKey string
				if common.Unmarshal(data[stringStart-1:index+1], &decodedKey) == nil {
					key = decodedKey
				}
				cursor := index + 1
				for cursor < len(data) && (data[cursor] == ' ' || data[cursor] == '\t' || data[cursor] == '\r' || data[cursor] == '\n') {
					cursor++
				}
				if cursor < len(data) && data[cursor] == ':' && key == target {
					count++
				}
				expectingKey = false
			}
			continue
		}
		switch current {
		case '"':
			inString = true
			stringStart = index + 1
		case '{':
			depth++
			if depth == 1 {
				expectingKey = true
			}
		case '}':
			depth--
		case ',':
			if depth == 1 {
				expectingKey = true
			}
		}
	}
	return count
}

func getJSONStringValue(result gjson.Result, field string) (string, error) {
	if !result.Exists() || result.Type == gjson.Null {
		return "", nil
	}
	if result.Type != gjson.String {
		return "", fmt.Errorf("field %s must be a string", field)
	}
	return result.String(), nil
}

func isModelAllowedByUser(c *gin.Context, modelName string) bool {
	userId := c.GetInt("id")
	if userId <= 0 {
		return true
	}
	userSetting, err := model.GetUserSetting(userId, false)
	if err != nil || !userSetting.ModelLimitsEnabled {
		return true
	}
	userModelLimit := model.BuildUserModelLimitMap(model.NormalizeUserModelLimits(userSetting.ModelLimits))
	return model.IsModelAllowedByUserLimit(modelName, userModelLimit)
}

func getModelRequest(c *gin.Context) (*ModelRequest, bool, error) {
	var modelRequest ModelRequest
	shouldSelectChannel := true
	var err error
	if modelName := c.GetString("resolved_task_model"); modelName != "" {
		modelRequest.Model = modelName
	} else if strings.Contains(c.Request.URL.Path, "/mj/") {
		relayMode := relayconstant.Path2RelayModeMidjourney(c.Request.URL.Path)
		if relayMode == relayconstant.RelayModeMidjourneyTaskFetch ||
			relayMode == relayconstant.RelayModeMidjourneyTaskFetchByCondition ||
			relayMode == relayconstant.RelayModeMidjourneyNotify ||
			relayMode == relayconstant.RelayModeMidjourneyTaskImageSeed {
			shouldSelectChannel = false
		} else {
			midjourneyRequest := taskdto.MidjourneyRequest{}
			err = common.UnmarshalBodyReusable(c, &midjourneyRequest)
			if err != nil {
				return nil, false, errors.New(i18n.T(c, i18n.MsgDistributorInvalidMidjourney, map[string]any{"Error": err.Error()}))
			}
			midjourneyModel, mjErr, success := service.GetMjRequestModel(relayMode, &midjourneyRequest)
			if mjErr != nil {
				return nil, false, fmt.Errorf("%s", mjErr.Description)
			}
			if midjourneyModel == "" {
				if !success {
					return nil, false, fmt.Errorf("%s", i18n.T(c, i18n.MsgDistributorInvalidParseModel))
				} else {
					// task fetch, task fetch by condition, notify
					shouldSelectChannel = false
				}
			}
			modelRequest.Model = midjourneyModel
		}
		c.Set("relay_mode", relayMode)
	} else if (strings.Contains(c.Request.URL.Path, "/v1/videos/") || strings.Contains(c.Request.URL.Path, "/pg/videos/")) && strings.HasSuffix(c.Request.URL.Path, "/remix") {
		relayMode := relayconstant.RelayModeVideoSubmit
		c.Set("relay_mode", relayMode)
		shouldSelectChannel = false
	} else if strings.Contains(c.Request.URL.Path, "/v1/videos") || strings.Contains(c.Request.URL.Path, "/pg/videos") {
		//curl https://api.openai.com/v1/videos \
		//  -H "Authorization: Bearer $OPENAI_API_KEY" \
		//  -F "model=sora-2" \
		//  -F "prompt=A calico cat playing a piano on stage"
		//	-F input_reference="@image.jpg"
		relayMode := relayconstant.RelayModeUnknown
		if c.Request.Method == http.MethodPost {
			relayMode = relayconstant.RelayModeVideoSubmit
			req, err := getModelFromRequest(c)
			if err != nil {
				return nil, false, err
			}
			if req != nil {
				modelRequest.Model = req.Model
			}
		} else if c.Request.Method == http.MethodGet {
			relayMode = relayconstant.RelayModeVideoFetchByID
			shouldSelectChannel = false
			modelRequest.Model = getTaskOriginModelName(c)
		}
		c.Set("relay_mode", relayMode)
	} else if strings.Contains(c.Request.URL.Path, "/v1/video/generations") {
		relayMode := relayconstant.RelayModeUnknown
		if c.Request.Method == http.MethodPost {
			req, err := getModelFromRequest(c)
			if err != nil {
				return nil, false, err
			}
			modelRequest.Model = req.Model
			relayMode = relayconstant.RelayModeVideoSubmit
		} else if c.Request.Method == http.MethodGet {
			relayMode = relayconstant.RelayModeVideoFetchByID
			shouldSelectChannel = false
			modelRequest.Model = getTaskOriginModelName(c)
		}
		if _, ok := c.Get("relay_mode"); !ok {
			c.Set("relay_mode", relayMode)
		}
	} else if strings.HasPrefix(c.Request.URL.Path, "/v1beta/models/") || strings.HasPrefix(c.Request.URL.Path, "/v1/models/") {
		// Gemini API 路径处理: /v1beta/models/gemini-2.0-flash:generateContent
		relayMode := relayconstant.RelayModeGemini
		modelName := extractModelNameFromGeminiPath(c.Request.URL.Path)
		if modelName != "" {
			modelRequest.Model = modelName
		}
		c.Set("relay_mode", relayMode)
	} else if !isAudioTranscriptionPath(c.Request.URL.Path) && !strings.Contains(c.Request.Header.Get("Content-Type"), "multipart/form-data") {
		req, err := getModelFromRequest(c)
		if err != nil {
			return nil, false, err
		}
		modelRequest.Model = req.Model
	}
	if strings.HasPrefix(c.Request.URL.Path, "/v1/realtime") {
		//wss://api.openai.com/v1/realtime?model=gpt-4o-realtime-preview-2024-10-01
		modelRequest.Model = c.Query("model")
	}
	if strings.HasPrefix(c.Request.URL.Path, "/v1/moderations") {
		if modelRequest.Model == "" {
			modelRequest.Model = "text-moderation-stable"
		}
	}
	if strings.HasSuffix(c.Request.URL.Path, "embeddings") {
		if modelRequest.Model == "" {
			modelRequest.Model = c.Param("model")
		}
	}
	if strings.HasPrefix(c.Request.URL.Path, "/v1/images/generations") {
		modelRequest.Model = common.GetStringIfEmpty(modelRequest.Model, "dall-e")
	} else if strings.HasPrefix(c.Request.URL.Path, "/v1/images/edits") {
		//modelRequest.Model = common.GetStringIfEmpty(c.PostForm("model"), "gpt-image-1")
		contentType := c.ContentType()
		if slices.Contains([]string{gin.MIMEPOSTForm, gin.MIMEMultipartPOSTForm}, contentType) {
			req, err := getModelFromRequest(c)
			if err == nil && req.Model != "" {
				modelRequest.Model = req.Model
			}
		}
	}
	if strings.HasPrefix(c.Request.URL.Path, "/v1/audio") || strings.HasPrefix(c.Request.URL.Path, "/pg/audio") || strings.HasPrefix(c.Request.URL.Path, "/v1/stt") {
		relayMode := relayconstant.RelayModeAudioSpeech
		if strings.HasPrefix(c.Request.URL.Path, "/v1/audio/speech") || strings.HasPrefix(c.Request.URL.Path, "/pg/audio/speech") {

			modelRequest.Model = common.GetStringIfEmpty(modelRequest.Model, "tts-1")
		} else if strings.HasPrefix(c.Request.URL.Path, "/v1/audio/translations") {
			// 先尝试从请求读取
			if req, err := getModelFromRequest(c); err == nil && req.Model != "" {
				modelRequest.Model = req.Model
			}
			modelRequest.Model = common.GetStringIfEmpty(modelRequest.Model, "whisper-1")
			relayMode = relayconstant.RelayModeAudioTranslation
		} else if isAudioTranscriptionPath(c.Request.URL.Path) {
			// 先尝试从请求读取
			if req, err := getModelFromRequest(c); err == nil && req.Model != "" {
				modelRequest.Model = req.Model
				modelRequest.Group = req.Group
			}
			defaultModel := "whisper-1"
			if strings.HasPrefix(c.Request.URL.Path, "/v1/stt") {
				defaultModel = "grok-stt"
			}
			modelRequest.Model = common.GetStringIfEmpty(modelRequest.Model, defaultModel)
			relayMode = relayconstant.RelayModeAudioTranscription
		}
		c.Set("relay_mode", relayMode)
	}
	if strings.HasPrefix(c.Request.URL.Path, "/pg/chat/completions") {
		// playground chat completions
		req, err := getModelFromRequest(c)
		if err != nil {
			return nil, false, err
		}
		modelRequest.Model = req.Model
		modelRequest.Group = req.Group
		common.SetContextKey(c, constant.ContextKeyTokenGroup, modelRequest.Group)
	}

	if strings.HasPrefix(c.Request.URL.Path, "/v1/responses/compact") && modelRequest.Model != "" {
		modelRequest.Model = ratio_setting.WithCompactModelSuffix(modelRequest.Model)
	}
	return &modelRequest, shouldSelectChannel, nil
}

func isAudioTranscriptionPath(path string) bool {
	return strings.HasPrefix(path, "/v1/audio/transcriptions") || strings.HasPrefix(path, "/pg/audio/transcriptions") || strings.HasPrefix(path, "/v1/stt")
}

// 修复 #4834: GET /v1/video/generations/:task_id && /v1/video/:task_id 此前不解析 model，
// 当 token 启用「可用模型限制」时，下游 modelLimitEnable 校验会因
// modelRequest.Model 为空而误报 "This token has no access to model"。
// 从已存储的任务记录中回填 OriginModelName 即可让校验走在正确的模型上。
func getTaskOriginModelName(c *gin.Context) string {
	if !common.GetContextKeyBool(c, constant.ContextKeyTokenModelLimitEnabled) {
		return ""
	}

	taskId := c.Param("task_id")
	if taskId == "" {
		return ""
	}

	userId := c.GetInt("id")
	if task, exist, err := model.GetByTaskId(userId, taskId); err == nil && exist && task != nil {
		return task.Properties.OriginModelName
	}
	return ""
}

func SetupContextForSelectedChannel(c *gin.Context, channel *model.Channel, modelName string) *types.NewAPIError {
	c.Set("original_model", modelName) // for retry
	expectedPlugin := c.GetString("expected_task_plugin_key")
	if channel == nil {
		logTaskPluginChannelDecision(c, nil, modelName, "channel_rejected", "nil_channel")
		return types.NewError(errors.New("channel is nil"), types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
	}
	if expectedPlugin != "" && !channelMatchesExpectedTaskPlugin(c, channel, expectedPlugin) {
		logTaskPluginChannelDecision(c, channel, modelName, "channel_rejected", "identity_mismatch")
		return types.NewError(
			errors.New("selected channel does not match the pinned task plugin"),
			types.ErrorCodeGetChannelFailed,
			types.ErrOptionWithSkipRetry(),
		)
	}
	if candidate, matched := pinnedEndpointCandidateForChannel(c, channel, expectedPlugin); matched {
		if value, exists := c.Get(jsplugin.ContextKeyPinnedEndpoint); exists {
			if pinned, ok := value.(jsplugin.PinnedEndpoint); ok && candidate.Plugin != nil && candidate.Plugin != pinned.Plugin {
				previousPlugin := pinned.Plugin.Meta.Key
				pinned.Plugin = candidate.Plugin
				pinned.Protocol = candidate.Protocol
				pinned.Operation = candidate.Operation
				c.Set(jsplugin.ContextKeyPinnedEndpoint, pinned)
				c.Set(jsplugin.ContextKeyPinnedPlugin, jsplugin.PinnedPlugin{Generation: pinned.Generation, Plugin: candidate.Plugin})
				c.Set("expected_task_plugin_key", candidate.Plugin.Meta.Key)
				c.Set("task_plugin_key", candidate.Plugin.Meta.Key)
				c.Set("platform", candidate.Plugin.Meta.Key)
				logger.LogDebug(
					c,
					"task_plugin subsystem=endpoint event=provider_selected generation=%d previous_plugin=%q plugin=%q model=%q channel_id=%d channel_type=%d",
					pinned.Generation.Number,
					previousPlugin,
					candidate.Plugin.Meta.Key,
					modelName,
					channel.Id,
					channel.Type,
				)
			}
		}
	}
	common.SetContextKey(c, constant.ContextKeyChannelId, channel.Id)
	common.SetContextKey(c, constant.ContextKeyChannelName, channel.Name)
	common.SetContextKey(c, constant.ContextKeyChannelType, channel.Type)
	common.SetContextKey(c, constant.ContextKeyChannelCreateTime, channel.CreatedTime)
	common.SetContextKey(c, constant.ContextKeyChannelSetting, channel.GetSetting())
	common.SetContextKey(c, constant.ContextKeyChannelOtherSetting, channel.GetOtherSettings())
	if channel.Type == constant.ChannelTypeTaskPlugin {
		c.Set("task_plugin_key", channel.GetSetting().TaskPluginKey)
	}
	logTaskPluginChannelDecision(c, channel, modelName, "channel_selected", "")
	paramOverride := channel.GetParamOverride()
	headerOverride := channel.GetHeaderOverride()
	if mergedParam, applied := service.ApplyChannelAffinityOverrideTemplate(c, paramOverride); applied {
		paramOverride = mergedParam
	}
	common.SetContextKey(c, constant.ContextKeyChannelParamOverride, paramOverride)
	common.SetContextKey(c, constant.ContextKeyChannelHeaderOverride, headerOverride)
	if nil != channel.OpenAIOrganization && *channel.OpenAIOrganization != "" {
		common.SetContextKey(c, constant.ContextKeyChannelOrganization, *channel.OpenAIOrganization)
	}
	common.SetContextKey(c, constant.ContextKeyChannelAutoBan, channel.GetAutoBan())
	common.SetContextKey(c, constant.ContextKeyChannelModelMapping, channel.GetModelMapping())
	common.SetContextKey(c, constant.ContextKeyChannelStatusCodeMapping, channel.GetStatusCodeMapping())

	key := ""
	index := 0
	if overrideValue, hasOverride := common.GetContextKey(c, constant.ContextKeyChannelKeyIndexOverride); hasOverride {
		var ok bool
		index, ok = overrideValue.(int)
		keys := channel.GetKeys()
		if !ok || index < 0 || index >= len(keys) {
			return types.NewError(errors.New("cursor channel: saved key index is unavailable"), types.ErrorCodeChannelNoAvailableKey, types.ErrOptionWithSkipRetry())
		}
		if status, exists := channel.ChannelInfo.MultiKeyStatusList[index]; exists && status != common.ChannelStatusEnabled {
			return types.NewError(errors.New("cursor channel: saved key is disabled"), types.ErrorCodeChannelNoAvailableKey, types.ErrOptionWithSkipRetry())
		}
		key = keys[index]
	} else {
		var newAPIError *types.NewAPIError
		key, index, newAPIError = channel.GetNextEnabledKey()
		if newAPIError != nil {
			return newAPIError
		}
	}
	if channel.ChannelInfo.IsMultiKey {
		common.SetContextKey(c, constant.ContextKeyChannelIsMultiKey, true)
		common.SetContextKey(c, constant.ContextKeyChannelMultiKeyIndex, index)
	} else {
		// 必须设置为 false，否则在重试到单个 key 的时候会导致日志显示错误
		common.SetContextKey(c, constant.ContextKeyChannelIsMultiKey, false)
	}
	// c.Request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", key))
	common.SetContextKey(c, constant.ContextKeyChannelKey, key)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, channel.GetBaseURL())

	common.SetContextKey(c, constant.ContextKeySystemPromptOverride, false)

	// TODO: api_version统一
	switch channel.Type {
	case constant.ChannelTypeAzure:
		c.Set("api_version", channel.Other)
	case constant.ChannelTypeVertexAi:
		c.Set("region", channel.Other)
	case constant.ChannelTypeXunfei:
		c.Set("api_version", channel.Other)
	case constant.ChannelTypeGemini:
		c.Set("api_version", channel.Other)
	case constant.ChannelTypeAli:
		c.Set("plugin", channel.Other)
	case constant.ChannelCloudflare:
		c.Set("api_version", channel.Other)
	case constant.ChannelTypeMokaAI:
		c.Set("api_version", channel.Other)
	case constant.ChannelTypeCoze:
		c.Set("bot_id", channel.Other)
	}
	return nil
}

// extractModelNameFromGeminiPath 从 Gemini API URL 路径中提取模型名
// 输入格式: /v1beta/models/gemini-2.0-flash:generateContent
// 输出: gemini-2.0-flash
func extractModelNameFromGeminiPath(path string) string {
	// 查找 "/models/" 的位置
	modelsPrefix := "/models/"
	modelsIndex := strings.Index(path, modelsPrefix)
	if modelsIndex == -1 {
		return ""
	}

	// 从 "/models/" 之后开始提取
	startIndex := modelsIndex + len(modelsPrefix)
	if startIndex >= len(path) {
		return ""
	}

	// 查找 ":" 的位置，模型名在 ":" 之前
	colonIndex := strings.Index(path[startIndex:], ":")
	if colonIndex == -1 {
		// 如果没有找到 ":"，返回从 "/models/" 到路径结尾的部分
		return path[startIndex:]
	}

	// 返回模型名部分
	return path[startIndex : startIndex+colonIndex]
}

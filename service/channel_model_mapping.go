package service

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	hostreasoning "github.com/QuantumNous/new-api/setting/reasoning"
	"github.com/gin-gonic/gin"
)

// ResolveChannelModelMapping 在安装渠道连接和凭证之前解析跨渠道映射。
// 原始请求模型继续用于权限和计费；每个目标渠道必须满足当前用户、分组及协议约束。
func ResolveChannelModelMapping(c *gin.Context, channel *model.Channel, requestModel, group string) (*model.Channel, *types.NewAPIError) {
	if channel == nil {
		return channel, nil
	}
	constraints := GetChannelConstraints(c)
	pin, pinned, _ := constraints.ResolvedPin()
	if pinned && pin.Source == dto.PinSourceOriginTask {
		return channel, nil
	}
	sourceID := channel.Id
	current := strings.TrimSuffix(requestModel, ratio_setting.CompactModelSuffix)
	visited := make(map[string]bool)
	routed := false
	for range 32 {
		routes := channel.GetSetting().ModelMappingChannels
		if len(routes) == 0 {
			if routed {
				common.SetContextKey(c, constant.ContextKeyModelMappingTarget, current)
				common.SetContextKey(c, constant.ContextKeyModelMappingSourceChannel, sourceID)
			}
			return channel, nil
		}
		var mapping map[string]string
		if err := common.UnmarshalJsonStr(channel.GetModelMapping(), &mapping); err != nil {
			return nil, types.NewErrorWithStatusCode(errors.New("跨渠道模型映射格式无效"), types.ErrorCodeChannelModelMappedError, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
		}
		key := current
		if mapping[key] == "" {
			key = hostreasoning.BaseModelName(current)
		}
		mapped := mapping[key]
		if mapped == "" || mapped == current && routes[key] == 0 {
			if routed {
				common.SetContextKey(c, constant.ContextKeyModelMappingTarget, current)
				common.SetContextKey(c, constant.ContextKeyModelMappingSourceChannel, sourceID)
			}
			return channel, nil
		}
		identity := fmt.Sprintf("%d:%s", channel.Id, current)
		if visited[identity] {
			return nil, types.NewErrorWithStatusCode(errors.New("跨渠道模型映射存在循环"), types.ErrorCodeChannelModelMappedError, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
		}
		visited[identity] = true
		targetID := routes[key]
		if targetID == 0 {
			current = mapped
			continue
		}
		if pinned && pin.ChannelId != targetID {
			return nil, types.NewErrorWithStatusCode(errors.New("跨渠道模型映射不能更改令牌指定的渠道"), types.ErrorCodeAccessDenied, http.StatusForbidden, types.ErrOptionWithSkipRetry())
		}
		target, err := model.CacheGetChannel(targetID)
		if err != nil || target == nil || target.Status != common.ChannelStatusEnabled {
			return nil, types.NewErrorWithStatusCode(errors.New("跨渠道模型映射的目标渠道不可用"), types.ErrorCodeGetChannelFailed, http.StatusServiceUnavailable, types.ErrOptionWithSkipRetry())
		}
		filters := append([]dto.ChannelFilter(nil), constraints.Filters...)
		filters = append(filters, dto.ChannelFilter{Kind: dto.FilterUserAccess, UserId: c.GetInt("id")}, dto.ChannelFilter{Kind: dto.FilterRequestPath, RequestPath: c.Request.URL.Path})
		if ok, _ := model.ChannelSatisfiesFilters(target, mapped, filters); !ok {
			return nil, types.NewErrorWithStatusCode(errors.New("跨渠道模型映射的目标渠道不满足访问或协议约束"), types.ErrorCodeAccessDenied, http.StatusForbidden, types.ErrOptionWithSkipRetry())
		}
		selectedGroup := group
		if selectedGroup == "" {
			selectedGroup = common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
		}
		if selectedGroup == "auto" {
			selectedGroup = ""
			for _, candidate := range GetRequestAutoGroups(c, common.GetContextKeyString(c, constant.ContextKeyUserGroup)) {
				if model.IsChannelEnabledForGroupModel(candidate, current, channel.Id) && model.IsChannelEnabledForGroupModel(candidate, mapped, targetID) {
					selectedGroup = candidate
					common.SetContextKey(c, constant.ContextKeyAutoGroup, candidate)
					break
				}
			}
		}
		if !model.IsChannelEnabledForGroupModel(selectedGroup, mapped, targetID) {
			return nil, types.NewErrorWithStatusCode(errors.New("跨渠道模型映射的目标模型未在当前分组开放"), types.ErrorCodeAccessDenied, http.StatusForbidden, types.ErrOptionWithSkipRetry())
		}
		channel, current, group, routed = target, mapped, selectedGroup, true
	}
	return nil, types.NewErrorWithStatusCode(errors.New("跨渠道模型映射链过长"), types.ErrorCodeChannelModelMappedError, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
}

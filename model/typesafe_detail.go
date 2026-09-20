package model

import (
	"slices"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

type TypeSafeDetailBody struct {
	Body      string `json:"body"`
	Bytes     int    `json:"bytes,omitempty"`
	Truncated bool   `json:"truncated,omitempty"`
}

type TypeSafeEvaluationDetail struct {
	*TypeSafeUserStage
	Stage    string              `json:"stage"`
	Request  *TypeSafeDetailBody `json:"request,omitempty"`
	Response *TypeSafeDetailBody `json:"response,omitempty"`
}

// 只解析详情所需字段，避免返回管理日志中的其他信息。
type typeSafeLinkedStage struct {
	TypeSafeUserStage
	Stage           string `json:"stage"`
	RequestID       string `json:"request_id"`
	ChannelID       int    `json:"channel_id"`
	ParentRequestID string `json:"parent_request_id"`
	ParentChannelID int    `json:"parent_channel_id"`
}

type typeSafeDetailMetadata struct {
	Results []typeSafeLinkedStage `json:"typesafe"`
	Admin   struct {
		Results  []typeSafeLinkedStage `json:"typesafe"`
		Exchange *struct {
			Request  *TypeSafeDetailBody `json:"request"`
			Response *TypeSafeDetailBody `json:"response"`
		} `json:"typesafe_exchange"`
	} `json:"admin_info"`
}

func GetUserTypeSafeEvaluationDetail(userID int, requestID, stage string, visibleChannelIDs []int) (*TypeSafeEvaluationDetail, error) {
	if requestID == "" || (stage != typeSafeStageBefore && stage != typeSafeStageAfter) {
		return nil, gorm.ErrRecordNotFound
	}
	var parent Log
	tx := LOG_DB.Where("user_id = ? AND request_id = ? AND type = ?", userID, requestID, LogTypeConsume)
	tx = applyResolvedLogChannelFilter(tx, "channel_id", visibleChannelIDs, true)
	if err := tx.First(&parent).Error; err != nil {
		return nil, err
	}
	var parentMeta typeSafeDetailMetadata
	if err := common.UnmarshalJsonStr(parent.Other, &parentMeta); err != nil {
		return nil, err
	}
	results := parentMeta.Admin.Results
	if len(results) == 0 {
		results = parentMeta.Results
	}
	var selected *typeSafeLinkedStage
	for i := range results {
		if results[i].Stage == stage && results[i].ParentRequestID == "" {
			selected = &results[i]
		}
	}
	if selected == nil || (selected.ChannelID > 0 && !slices.Contains(visibleChannelIDs, selected.ChannelID)) {
		return nil, gorm.ErrRecordNotFound
	}
	detail := &TypeSafeEvaluationDetail{TypeSafeUserStage: &selected.TypeSafeUserStage, Stage: stage}
	if selected.RequestID == "" {
		// 旧日志只有评估摘要，不能据此重建输入输出。
		return detail, nil
	}
	var child Log
	tx = LOG_DB.Where("user_id = ? AND request_id = ? AND channel_id = ? AND type = ?", userID, selected.RequestID, selected.ChannelID, LogTypeConsume)
	tx = applyResolvedLogChannelFilter(tx, "channel_id", visibleChannelIDs, true)
	if err := tx.First(&child).Error; err != nil {
		return nil, err
	}
	var childMeta typeSafeDetailMetadata
	if err := common.UnmarshalJsonStr(child.Other, &childMeta); err != nil {
		return nil, err
	}
	for _, link := range childMeta.Admin.Results {
		if link.ParentRequestID != requestID || link.ParentChannelID != parent.ChannelId || link.Stage != stage {
			continue
		}
		if exchange := childMeta.Admin.Exchange; exchange != nil {
			detail.Request, detail.Response = exchange.Request, exchange.Response
		}
		return detail, nil
	}
	return nil, gorm.ErrRecordNotFound
}

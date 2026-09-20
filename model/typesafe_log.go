package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
)

const (
	typeSafeStageBefore = "before"
	typeSafeStageAfter  = "after"
)

type TypeSafeUserEvaluation struct {
	CreatedAt int64              `json:"created_at"`
	ModelName string             `json:"model_name"`
	TokenName string             `json:"token_name"`
	RequestId string             `json:"request_id"`
	Group     string             `json:"group,omitempty"`
	Before    *TypeSafeUserStage `json:"before,omitempty"`
	After     *TypeSafeUserStage `json:"after,omitempty"`
}

type TypeSafeUserStage struct {
	Model     string         `json:"model,omitempty"`
	Status    string         `json:"status"`
	Reason    string         `json:"reason,omitempty"`
	Truncated bool           `json:"truncated,omitempty"`
	Answers   map[string]any `json:"answers,omitempty"`
}

func SanitizeTypeSafeResults(raw any) []map[string]any {
	items := typeSafeResultItems(raw)
	if len(items) == 0 {
		return nil
	}
	summaries := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if _, hasParent := item["parent_request_id"]; hasParent {
			continue
		}
		summary := make(map[string]any, 6)
		if model, ok := item["model"].(string); ok && model != "" {
			summary["model"] = model
		}
		if stage, ok := item["stage"].(string); ok && stage != "" {
			summary["stage"] = stage
		}
		if status, ok := item["status"].(string); ok && status != "" {
			summary["status"] = status
		}
		if reason, ok := item["reason"].(string); ok && reason != "" {
			summary["reason"] = reason
		}
		if truncated, ok := item["truncated"].(bool); ok {
			summary["truncated"] = truncated
		}
		if answers, ok := item["answers"].(map[string]any); ok && len(answers) > 0 {
			copied := make(map[string]any, len(answers))
			for key, value := range answers {
				copied[key] = value
			}
			summary["answers"] = copied
		}
		if len(summary) == 0 {
			continue
		}
		summaries = append(summaries, summary)
	}
	if len(summaries) == 0 {
		return nil
	}
	return summaries
}

func promoteTypeSafeSummary(otherMap map[string]interface{}) {
	if otherMap == nil {
		return
	}
	delete(otherMap, "typesafe_exchange")
	raw := otherMap["typesafe"]
	// 原始评估记录保留模型信息；兼容旧日志中已过滤模型的公开摘要。
	if admin, ok := otherMap["admin_info"].(map[string]interface{}); ok && admin["typesafe"] != nil {
		raw = admin["typesafe"]
	}
	summary := SanitizeTypeSafeResults(raw)
	if len(summary) == 0 {
		delete(otherMap, "typesafe")
		return
	}
	otherMap["typesafe"] = summary
}

func GetUserTypeSafeEvaluations(userId int, startTimestamp int64, endTimestamp int64, modelName string, requestId string, startIdx int, num int, visibleChannelIDs []int) ([]*TypeSafeUserEvaluation, int64, error) {
	tx := LOG_DB.Where("logs.user_id = ? AND logs.type = ?", userId, LogTypeConsume)
	tx = tx.Where("(logs.other LIKE ? OR logs.other LIKE ?) AND logs.other NOT LIKE ?",
		`%"stage":"before"%`, `%"stage":"after"%`, `%"parent_request_id":%`)
	var err error
	if tx, err = applyLogSearchFilter(tx, "logs.model_name", modelName); err != nil {
		return nil, 0, err
	}
	if requestId != "" {
		tx = tx.Where("logs.request_id = ?", requestId)
	}
	if startTimestamp != 0 {
		tx = tx.Where("logs.created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("logs.created_at <= ?", endTimestamp)
	}
	tx = applyResolvedLogChannelFilter(tx, "logs.channel_id", visibleChannelIDs, true)

	var total int64
	if err = tx.Model(&Log{}).Limit(logSearchCountLimit).Count(&total).Error; err != nil {
		common.SysError("failed to count typesafe evaluations: " + err.Error())
		return nil, 0, errors.New("查询日志失败")
	}
	order := "logs.id desc"
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		order = clickHouseLogOrder("logs.")
	}
	var logs []*Log
	if err = tx.Order(order).Limit(num).Offset(startIdx).Find(&logs).Error; err != nil {
		common.SysError("failed to search typesafe evaluations: " + err.Error())
		return nil, 0, errors.New("查询日志失败")
	}

	formatUserLogs(logs, startIdx)
	items := make([]*TypeSafeUserEvaluation, 0, len(logs))
	for _, log := range logs {
		if item := buildTypeSafeUserEvaluation(log); item != nil {
			items = append(items, item)
		}
	}
	return items, total, nil
}

func buildTypeSafeUserEvaluation(log *Log) *TypeSafeUserEvaluation {
	if log == nil {
		return nil
	}
	otherMap, _ := common.StrToMap(log.Other)
	if otherMap == nil {
		return nil
	}
	evaluation := &TypeSafeUserEvaluation{
		CreatedAt: log.CreatedAt,
		ModelName: log.ModelName,
		TokenName: log.TokenName,
		RequestId: log.RequestId,
		Group:     log.Group,
	}
	for _, item := range SanitizeTypeSafeResults(otherMap["typesafe"]) {
		stage, _ := item["stage"].(string)
		summary := typeSafeUserStageFromMap(item)
		switch stage {
		case typeSafeStageBefore:
			evaluation.Before = summary
		case typeSafeStageAfter:
			evaluation.After = summary
		}
	}
	if evaluation.Before == nil && evaluation.After == nil {
		return nil
	}
	return evaluation
}

func typeSafeUserStageFromMap(item map[string]any) *TypeSafeUserStage {
	stage := &TypeSafeUserStage{}
	if model, ok := item["model"].(string); ok {
		stage.Model = model
	}
	if status, ok := item["status"].(string); ok {
		stage.Status = status
	}
	if reason, ok := item["reason"].(string); ok {
		stage.Reason = reason
	}
	if truncated, ok := item["truncated"].(bool); ok {
		stage.Truncated = truncated
	}
	if answers, ok := item["answers"].(map[string]any); ok && len(answers) > 0 {
		stage.Answers = answers
	}
	return stage
}

func typeSafeResultItems(raw any) []map[string]any {
	switch values := raw.(type) {
	case []map[string]any:
		return values
	case []any:
		items := make([]map[string]any, 0, len(values))
		for _, value := range values {
			item, ok := value.(map[string]any)
			if !ok || item == nil {
				continue
			}
			items = append(items, item)
		}
		return items
	default:
		return nil
	}
}

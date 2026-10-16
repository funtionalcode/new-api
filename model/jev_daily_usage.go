package model

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// 日报只读取既有计费日志，不改变结算；Jev 子请求不计入主请求次数。
type JevDailyUsage struct {
	RequestCount         int64 `json:"request_count"`
	PromptTokens         int64 `json:"prompt_tokens"`
	CompletionTokens     int64 `json:"completion_tokens"`
	Quota                int64 `json:"quota"`
	UseTimeSeconds       int64 `json:"use_time_seconds"`
	CachedDecisions      int64 `json:"cached_decisions"`
	EvaluationRequests   int64 `json:"evaluation_requests"`
	EvaluationPrompt     int64 `json:"evaluation_prompt_tokens"`
	EvaluationCompletion int64 `json:"evaluation_completion_tokens"`
	EvaluationQuota      int64 `json:"evaluation_quota"`
}

type JevDailyUsageRow struct {
	UserID    int    `json:"user_id"`
	Username  string `json:"username"`
	AuthIndex string `json:"auth_index"`
	AuthName  string `json:"auth_name"`
	Provider  string `json:"provider"`
	Bound     bool   `json:"bound"`
	ModelName string `json:"model_name"`
	JevStatus string `json:"jev_status"`
	JevDailyUsage
}

type JevDailyReport struct {
	StartTimestamp int64              `json:"start_timestamp"`
	EndTimestamp   int64              `json:"end_timestamp"`
	LogCount       int                `json:"log_count"`
	Rows           []JevDailyUsageRow `json:"rows"`
}

type jevUsageStage struct {
	Stage           string `json:"stage"`
	Model           string `json:"model"`
	Status          string `json:"status"`
	Source          string `json:"source"`
	Applied         *bool  `json:"applied"`
	ParentRequestID string `json:"parent_request_id"`
	ParentChannelID int    `json:"parent_channel_id"`
}

type jevUsageMetadata struct {
	Stages []jevUsageStage `json:"typesafe"`
	Admin  struct {
		Stages []jevUsageStage `json:"typesafe"`
	} `json:"admin_info"`
}

func isJevUsageModel(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	// 供应商命名空间可能有多级；只匹配最后一段，避免 other-jev-model 误分类。
	if index := strings.LastIndexByte(name, '/'); index >= 0 {
		name = name[index+1:]
	}
	return name == "jev" || strings.HasPrefix(name, "jev-")
}

func jevUsageCohort(meta jevUsageMetadata) (string, bool, *jevUsageStage) {
	stages := meta.Stages
	if len(meta.Admin.Stages) > 0 {
		stages = meta.Admin.Stages
	}
	cohort, cached := "not_used", false
	for _, stage := range stages {
		if stage.ParentRequestID != "" {
			return "evaluation_only", false, &stage
		}
		if stage.Model != "" && !isJevUsageModel(stage.Model) || stage.Model == "" && stage.Stage != "adaptive_reasoning" {
			continue
		}
		if stage.Applied != nil && *stage.Applied || stage.Status == "success" && stage.Stage != "adaptive_reasoning" {
			cohort = "used"
			cached = cached || stage.Source == "cache" || stage.Source == "request_retry"
		} else if cohort != "used" && stage.Status == "success" && stage.Applied == nil {
			cohort = "unknown"
		} else if cohort != "used" && cohort != "unknown" {
			cohort = "not_applied"
		}
	}
	return cohort, cached, nil
}

// GetJevDailyUsage 支持分离日志库，不对主库表做跨库 JOIN，也不依赖数据库 JSON 方言。
func GetJevDailyUsage(ctx context.Context, start, end int64) (*JevDailyReport, error) {
	if start <= 0 || end <= start || end-start > 26*3600 {
		return nil, errors.New("invalid daily time range")
	}
	const maxDailyLogs = 100_000
	tx := LOG_DB.WithContext(ctx).Model(&Log{}).
		Select("user_id, username, token_id, token_name, channel_id, model_name, request_id, prompt_tokens, completion_tokens, quota, use_time, other").
		Where("type = ? AND created_at >= ? AND created_at < ?", LogTypeConsume, start, end).
		Limit(maxDailyLogs + 1)
	cursor, err := tx.Rows()
	if err != nil {
		return nil, err
	}
	defer cursor.Close()
	result := &JevDailyReport{StartTimestamp: start, EndTimestamp: end, Rows: []JevDailyUsageRow{}}
	rows := make(map[string]*JevDailyUsageRow)
	parents := make(map[string]string)
	unclassifiedParents := make(map[string]JevDailyUsage)
	type evaluation struct {
		log    Log
		parent *jevUsageStage
	}
	evaluations := []evaluation{}
	for cursor.Next() {
		result.LogCount++
		if result.LogCount > maxDailyLogs {
			return nil, errors.New("daily report exceeds 100000 consumption logs; no partial totals returned")
		}
		var log Log
		if err := LOG_DB.ScanRows(cursor, &log); err != nil {
			return nil, err
		}
		var meta jevUsageMetadata
		cohort, cached := "not_used", false
		var parent *jevUsageStage
		if strings.TrimSpace(log.Other) != "" && log.Other != "null" {
			if common.UnmarshalJsonStr(log.Other, &meta) != nil {
				cohort = "unknown"
			} else {
				cohort, cached, parent = jevUsageCohort(meta)
			}
		}
		log.Other = ""
		if isJevUsageModel(log.ModelName) {
			evaluations = append(evaluations, evaluation{log: log, parent: parent})
			continue
		}
		key := fmt.Sprintf("%d\x00%s\x00%s\x00%s", log.UserId, log.TokenName, log.ModelName, cohort)
		row := rows[key]
		if row == nil {
			row = &JevDailyUsageRow{UserID: log.UserId, Username: log.Username, AuthIndex: log.TokenName, ModelName: log.ModelName, JevStatus: cohort}
			rows[key] = row
		}
		row.RequestCount++
		row.PromptTokens += int64(log.PromptTokens)
		row.CompletionTokens += int64(log.CompletionTokens)
		row.Quota += int64(log.Quota)
		row.UseTimeSeconds += int64(log.UseTime)
		if cached {
			row.CachedDecisions++
		}
		if log.RequestId != "" {
			parentKey := fmt.Sprintf("%d\x00%d\x00%d\x00%s", log.UserId, log.TokenId, log.ChannelId, log.RequestId)
			if _, exists := parents[parentKey]; exists {
				parents[parentKey] = "" // 同一标识对应多个请求时不猜测归属。
				delete(unclassifiedParents, parentKey)
			} else {
				parents[parentKey] = key
				if cohort == "not_used" {
					unclassifiedParents[parentKey] = JevDailyUsage{RequestCount: 1, PromptTokens: int64(log.PromptTokens), CompletionTokens: int64(log.CompletionTokens), Quota: int64(log.Quota), UseTimeSeconds: int64(log.UseTime)}
				}
			}
		}
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	for _, item := range evaluations {
		log := item.log
		var row *JevDailyUsageRow
		if item.parent != nil {
			parentKey := fmt.Sprintf("%d\x00%d\x00%d\x00%s", log.UserId, log.TokenId, item.parent.ParentChannelID, item.parent.ParentRequestID)
			row = rows[parents[parentKey]]
			if row != nil && row.AuthIndex != log.TokenName {
				row = nil
			}
			if row != nil && row.JevStatus == "not_used" {
				if usage, exists := unclassifiedParents[parentKey]; exists {
					// 子评估证明发生过 Jev 调用，但不能补证父请求缺失的应用状态。
					// 只迁移匹配的一个父请求，不改变同文件其他无 Jev 记录的请求。
					row.RequestCount -= usage.RequestCount
					row.PromptTokens -= usage.PromptTokens
					row.CompletionTokens -= usage.CompletionTokens
					row.Quota -= usage.Quota
					row.UseTimeSeconds -= usage.UseTimeSeconds
					oldKey := parents[parentKey]
					key := fmt.Sprintf("%d\x00%s\x00%s\x00unknown", row.UserID, row.AuthIndex, row.ModelName)
					unknown := rows[key]
					if unknown == nil {
						unknown = &JevDailyUsageRow{UserID: row.UserID, Username: row.Username, AuthIndex: row.AuthIndex, ModelName: row.ModelName, JevStatus: "unknown"}
						rows[key] = unknown
					}
					unknown.RequestCount += usage.RequestCount
					unknown.PromptTokens += usage.PromptTokens
					unknown.CompletionTokens += usage.CompletionTokens
					unknown.Quota += usage.Quota
					unknown.UseTimeSeconds += usage.UseTimeSeconds
					if row.RequestCount == 0 {
						delete(rows, oldKey)
					}
					row = unknown
					parents[parentKey] = key
					delete(unclassifiedParents, parentKey)
				}
			}
		}
		if row == nil {
			key := fmt.Sprintf("%d\x00%s\x00evaluation_only", log.UserId, log.TokenName)
			row = rows[key]
			if row == nil {
				row = &JevDailyUsageRow{UserID: log.UserId, Username: log.Username, AuthIndex: log.TokenName, JevStatus: "evaluation_only"}
				rows[key] = row
			}
		}
		row.EvaluationRequests++
		row.EvaluationPrompt += int64(log.PromptTokens)
		row.EvaluationCompletion += int64(log.CompletionTokens)
		row.EvaluationQuota += int64(log.Quota)
	}
	// 仅取展示与归属字段，避免加载凭据或返回认证文件内容；无请求的绑定也显示为零。
	var bindings []struct {
		UserId    int
		Username  string
		AuthIndex string
		AuthName  string
		Provider  string
	}
	if err := DB.WithContext(ctx).Model(&CliproxyAuthFileBinding{}).
		Select("user_id, username, auth_index, auth_name, provider").Find(&bindings).Error; err != nil {
		return nil, err
	}
	activeBindings := make(map[string]bool)
	for _, row := range rows {
		activeBindings[userTokenUsageBindingKey(row.UserID, row.AuthIndex)] = true
	}
	for _, binding := range bindings {
		bindingKey := userTokenUsageBindingKey(binding.UserId, binding.AuthIndex)
		if !activeBindings[bindingKey] {
			rows[bindingKey+"\x00no_activity"] = &JevDailyUsageRow{UserID: binding.UserId, Username: binding.Username, AuthIndex: binding.AuthIndex, JevStatus: "no_activity"}
		}
	}
	byBinding := make(map[string]int, len(bindings))
	for i, binding := range bindings {
		byBinding[userTokenUsageBindingKey(binding.UserId, binding.AuthIndex)] = i
	}
	for _, row := range rows {
		if i, ok := byBinding[userTokenUsageBindingKey(row.UserID, row.AuthIndex)]; ok {
			binding := bindings[i]
			row.Bound, row.AuthName, row.Provider = true, binding.AuthName, binding.Provider
			if row.Username == "" {
				row.Username = binding.Username
			}
		}
		result.Rows = append(result.Rows, *row)
	}
	slices.SortFunc(result.Rows, func(a, b JevDailyUsageRow) int {
		if order := cmp.Compare(a.UserID, b.UserID); order != 0 {
			return order
		}
		if order := strings.Compare(a.AuthIndex, b.AuthIndex); order != 0 {
			return order
		}
		if order := strings.Compare(a.ModelName, b.ModelName); order != 0 {
			return order
		}
		return strings.Compare(a.JevStatus, b.JevStatus)
	})
	return result, nil
}

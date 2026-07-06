package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

type CliproxyAuthFileBinding struct {
	Id                       int    `json:"id" gorm:"primaryKey"`
	UserId                   int    `json:"user_id" gorm:"index;not null"`
	Username                 string `json:"username" gorm:"size:64;index;default:''"`
	AuthIndex                string `json:"auth_index" gorm:"size:128;uniqueIndex;not null"`
	AuthName                 string `json:"auth_name" gorm:"size:255;default:''"`
	AuthFile                 string `json:"auth_file" gorm:"type:text"`
	Note                     string `json:"note" gorm:"type:text"`
	AccountId                string `json:"account_id" gorm:"size:128;index;default:''"`
	Enabled                  bool   `json:"enabled" gorm:"default:true"`
	LastRefreshedAt          int64  `json:"last_refreshed_at" gorm:"bigint;default:0"`
	LastUsageTokens          int    `json:"last_usage_tokens" gorm:"default:0"`
	LastUsageQuota           int    `json:"last_usage_quota" gorm:"default:0"`
	LastPlanType             string `json:"last_plan_type" gorm:"size:64;default:''"`
	LastFiveHourPercent      int    `json:"last_five_hour_percent" gorm:"default:0"`
	LastFiveHourResetAt      int64  `json:"last_five_hour_reset_at" gorm:"bigint;default:0"`
	LastWeeklyPercent        int    `json:"last_weekly_percent" gorm:"default:0"`
	LastWeeklyResetAt        int64  `json:"last_weekly_reset_at" gorm:"bigint;default:0"`
	LastCodexFiveHourPercent int    `json:"last_codex_five_hour_percent" gorm:"default:0"`
	LastCodexFiveHourResetAt int64  `json:"last_codex_five_hour_reset_at" gorm:"bigint;default:0"`
	LastCodexWeeklyPercent   int    `json:"last_codex_weekly_percent" gorm:"default:0"`
	LastCodexWeeklyResetAt   int64  `json:"last_codex_weekly_reset_at" gorm:"bigint;default:0"`
	LastError                string `json:"last_error" gorm:"type:text"`
	CreatedAt                int64  `json:"created_at" gorm:"bigint;index"`
	UpdatedAt                int64  `json:"updated_at" gorm:"bigint"`
}

type CliproxyAuthFileBindingQuery struct {
	UserId    int
	Username  string
	AuthIndex string
	Enabled   *bool
}

type CliproxyAuthFileBindingUpdate struct {
	UserId       int
	Username     string
	AuthIndex    string
	AuthName     string
	AuthFile     string
	Note         string
	AccountId    string
	LastPlanType string
	Enabled      bool
}

type CliproxyUsageRefreshUpdate struct {
	LastUsageTokens          int
	LastUsageQuota           int
	LastPlanType             string
	LastFiveHourPercent      int
	LastFiveHourResetAt      int64
	LastWeeklyPercent        int
	LastWeeklyResetAt        int64
	LastCodexFiveHourPercent int
	LastCodexFiveHourResetAt int64
	LastCodexWeeklyPercent   int
	LastCodexWeeklyResetAt   int64
	LastError                string
}

type UserTokenUsageSummary struct {
	UserId           int    `json:"user_id"`
	Username         string `json:"username"`
	Remark           string `json:"remark"`
	TokenId          int    `json:"token_id"`
	TokenName        string `json:"token_name"`
	AuthIndex        string `json:"auth_index"`
	AuthName         string `json:"auth_name"`
	ChannelId        int    `json:"channel_id"`
	ChannelName      string `json:"channel_name"`
	ModelName        string `json:"model_name"`
	RequestCount     int64  `json:"request_count"`
	PromptTokens     int64  `json:"prompt_tokens"`
	CompletionTokens int64  `json:"completion_tokens"`
	TotalTokens      int64  `json:"total_tokens"`
	Quota            int64  `json:"quota"`
	LastCalledAt     int64  `json:"last_called_at"`
}

type UserTokenDailyUsage struct {
	Day              int64  `json:"day"`
	UserId           int    `json:"user_id"`
	Username         string `json:"username"`
	Remark           string `json:"remark"`
	TokenId          int    `json:"token_id"`
	TokenName        string `json:"token_name"`
	ChannelId        int    `json:"channel_id"`
	ChannelName      string `json:"channel_name"`
	ModelName        string `json:"model_name"`
	RequestCount     int64  `json:"request_count"`
	PromptTokens     int64  `json:"prompt_tokens"`
	CompletionTokens int64  `json:"completion_tokens"`
	TotalTokens      int64  `json:"total_tokens"`
	Quota            int64  `json:"quota"`
	LastCalledAt     int64  `json:"last_called_at"`
}

type UserTokenUsageQuery struct {
	StartTimestamp int64
	EndTimestamp   int64
	UserId         int
	Username       string
	TokenName      string
	AuthIndex      string
	ChannelId      int
	ModelName      string
	SortBy         string
	SortOrder      string
}

func (CliproxyAuthFileBinding) TableName() string {
	return "cliproxy_auth_file_bindings"
}

func (binding *CliproxyAuthFileBinding) BeforeCreate() error {
	now := time.Now().Unix()
	binding.CreatedAt = now
	binding.UpdatedAt = now
	return nil
}

func (binding *CliproxyAuthFileBinding) BeforeUpdate() error {
	binding.UpdatedAt = time.Now().Unix()
	return nil
}

func CreateCliproxyAuthFileBinding(binding *CliproxyAuthFileBinding) error {
	return DB.Create(binding).Error
}

func GetCliproxyAuthFileBindingById(id int) (*CliproxyAuthFileBinding, error) {
	var binding CliproxyAuthFileBinding
	err := DB.First(&binding, "id = ?", id).Error
	return &binding, err
}

func GetCliproxyAuthFileBindings(query CliproxyAuthFileBindingQuery, startIdx int, num int) ([]*CliproxyAuthFileBinding, int64, error) {
	var bindings []*CliproxyAuthFileBinding
	dbQuery := buildCliproxyAuthFileBindingQuery(query)
	var total int64
	if err := dbQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := dbQuery.Order(cliproxyAuthFileBindingOrderClause()).Limit(num).Offset(startIdx).Find(&bindings).Error
	return bindings, total, err
}

func cliproxyAuthFileBindingOrderClause() string {
	return "CASE lower(replace(replace(replace(last_plan_type, '-', ''), '_', ''), ' ', '')) WHEN 'pro' THEN 0 WHEN 'pro20x' THEN 0 WHEN 'planmax' THEN 0 WHEN 'claudemax' THEN 0 WHEN 'prolite' THEN 1 WHEN 'pro5x' THEN 1 WHEN 'team' THEN 2 WHEN 'planteam' THEN 2 WHEN 'claudeteam' THEN 2 WHEN 'plus' THEN 3 WHEN 'planpro' THEN 3 WHEN 'claudepro' THEN 3 WHEN 'free' THEN 4 WHEN 'planfree' THEN 4 WHEN 'claudefree' THEN 4 WHEN '' THEN 6 ELSE 5 END ASC, lower(last_plan_type) ASC, id DESC"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func buildCliproxyAuthFileBindingQuery(query CliproxyAuthFileBindingQuery) *gorm.DB {
	dbQuery := DB.Model(&CliproxyAuthFileBinding{})
	if query.UserId > 0 {
		dbQuery = dbQuery.Where("user_id = ?", query.UserId)
	}
	if strings.TrimSpace(query.Username) != "" {
		dbQuery = dbQuery.Where("username LIKE ?", "%"+strings.TrimSpace(query.Username)+"%")
	}
	if strings.TrimSpace(query.AuthIndex) != "" {
		dbQuery = dbQuery.Where("auth_index = ?", strings.TrimSpace(query.AuthIndex))
	}
	if query.Enabled != nil {
		dbQuery = dbQuery.Where("enabled = ?", *query.Enabled)
	}
	return dbQuery
}

func UpdateCliproxyAuthFileBinding(id int, update CliproxyAuthFileBindingUpdate) (*CliproxyAuthFileBinding, error) {
	binding, err := GetCliproxyAuthFileBindingById(id)
	if err != nil {
		return nil, err
	}
	updatedBinding := &CliproxyAuthFileBinding{
		Id:                       binding.Id,
		UserId:                   update.UserId,
		Username:                 update.Username,
		AuthIndex:                update.AuthIndex,
		AuthName:                 update.AuthName,
		AuthFile:                 update.AuthFile,
		Note:                     update.Note,
		AccountId:                update.AccountId,
		Enabled:                  update.Enabled,
		LastRefreshedAt:          binding.LastRefreshedAt,
		LastUsageTokens:          binding.LastUsageTokens,
		LastUsageQuota:           binding.LastUsageQuota,
		LastPlanType:             firstNonEmpty(update.LastPlanType, binding.LastPlanType),
		LastFiveHourPercent:      binding.LastFiveHourPercent,
		LastFiveHourResetAt:      binding.LastFiveHourResetAt,
		LastWeeklyPercent:        binding.LastWeeklyPercent,
		LastWeeklyResetAt:        binding.LastWeeklyResetAt,
		LastCodexFiveHourPercent: binding.LastCodexFiveHourPercent,
		LastCodexFiveHourResetAt: binding.LastCodexFiveHourResetAt,
		LastCodexWeeklyPercent:   binding.LastCodexWeeklyPercent,
		LastCodexWeeklyResetAt:   binding.LastCodexWeeklyResetAt,
		LastError:                binding.LastError,
		CreatedAt:                binding.CreatedAt,
		UpdatedAt:                time.Now().Unix(),
	}
	return updatedBinding, DB.Save(updatedBinding).Error
}

func UpdateCliproxyAuthFileBindingUsage(id int, update CliproxyUsageRefreshUpdate) (*CliproxyAuthFileBinding, error) {
	binding, err := GetCliproxyAuthFileBindingById(id)
	if err != nil {
		return nil, err
	}
	updatedBinding := &CliproxyAuthFileBinding{
		Id:                       binding.Id,
		UserId:                   binding.UserId,
		Username:                 binding.Username,
		AuthIndex:                binding.AuthIndex,
		AuthName:                 binding.AuthName,
		AuthFile:                 binding.AuthFile,
		Note:                     binding.Note,
		AccountId:                binding.AccountId,
		Enabled:                  binding.Enabled,
		LastRefreshedAt:          time.Now().Unix(),
		LastUsageTokens:          update.LastUsageTokens,
		LastUsageQuota:           update.LastUsageQuota,
		LastPlanType:             update.LastPlanType,
		LastFiveHourPercent:      update.LastFiveHourPercent,
		LastFiveHourResetAt:      update.LastFiveHourResetAt,
		LastWeeklyPercent:        update.LastWeeklyPercent,
		LastWeeklyResetAt:        update.LastWeeklyResetAt,
		LastCodexFiveHourPercent: update.LastCodexFiveHourPercent,
		LastCodexFiveHourResetAt: update.LastCodexFiveHourResetAt,
		LastCodexWeeklyPercent:   update.LastCodexWeeklyPercent,
		LastCodexWeeklyResetAt:   update.LastCodexWeeklyResetAt,
		LastError:                update.LastError,
		CreatedAt:                binding.CreatedAt,
		UpdatedAt:                time.Now().Unix(),
	}
	return updatedBinding, DB.Save(updatedBinding).Error
}

func DeleteCliproxyAuthFileBindingById(id int) error {
	return DB.Delete(&CliproxyAuthFileBinding{}, "id = ?", id).Error
}

func GetUserTokenUsageSummary(query UserTokenUsageQuery, startIdx int, num int) ([]*UserTokenUsageSummary, int64, error) {
	if userTokenUsageNeedsMainDBEnrichment() {
		return getUserTokenUsageSummaryFromSeparatedLogDB(query, startIdx, num)
	}

	groupClause := "logs.user_id, logs.token_id, logs.token_name, logs.channel_id, channels.name, logs.model_name"
	baseQuery := buildUserTokenUsageBaseQuery(query)
	var groups []struct {
		UserId      int
		TokenId     int
		TokenName   string
		ChannelId   int
		ChannelName string
		ModelName   string
	}
	if err := baseQuery.Select("logs.user_id, logs.token_id, logs.token_name, logs.channel_id, channels.name as channel_name, logs.model_name").Group(groupClause).Scan(&groups).Error; err != nil {
		return nil, 0, err
	}
	total := int64(len(groups))
	var summaries []*UserTokenUsageSummary
	selectClause := "logs.user_id, MAX(logs.username) AS username, coalesce(MAX(users.remark), '') AS remark, logs.token_id, logs.token_name, coalesce(max(cliproxy_auth_file_bindings.auth_index), '') as auth_index, coalesce(max(cliproxy_auth_file_bindings.auth_name), '') as auth_name, logs.channel_id, coalesce(channels.name, '') as channel_name, logs.model_name, count(*) as request_count, coalesce(sum(logs.prompt_tokens), 0) as prompt_tokens, coalesce(sum(logs.completion_tokens), 0) as completion_tokens, coalesce(sum(logs.prompt_tokens), 0) + coalesce(sum(logs.completion_tokens), 0) as total_tokens, coalesce(sum(logs.quota), 0) as quota, coalesce(max(logs.created_at), 0) as last_called_at"
	err := buildUserTokenUsageBaseQuery(query).
		Select(selectClause).
		Group(groupClause).
		Order(resolveUserTokenUsageOrder(query)).
		Limit(num).
		Offset(startIdx).
		Scan(&summaries).Error
	return summaries, total, err
}

func GetUserTokenUsageByDay(query UserTokenUsageQuery, startIdx int, num int) ([]*UserTokenDailyUsage, int64, error) {
	if userTokenUsageNeedsMainDBEnrichment() {
		return getUserTokenUsageByDayFromSeparatedLogDB(query, startIdx, num)
	}

	dayExpr := userTokenUsageDayExpr()
	baseQuery := buildUserTokenUsageBaseQuery(query)
	var groups []struct {
		Day         int64
		UserId      int
		TokenId     int
		TokenName   string
		ChannelId   int
		ChannelName string
		ModelName   string
	}
	groupClause := fmt.Sprintf("%s, logs.user_id, logs.token_id, logs.token_name, logs.channel_id, channels.name, logs.model_name", dayExpr)
	if err := baseQuery.Select(fmt.Sprintf("%s as day, logs.user_id, logs.token_id, logs.token_name, logs.channel_id, channels.name as channel_name, logs.model_name", dayExpr)).Group(groupClause).Scan(&groups).Error; err != nil {
		return nil, 0, err
	}
	total := int64(len(groups))
	var summaries []*UserTokenDailyUsage
	selectClause := fmt.Sprintf("%s as day, logs.user_id, MAX(logs.username) AS username, coalesce(MAX(users.remark), '') AS remark, logs.token_id, logs.token_name, logs.channel_id, coalesce(channels.name, '') as channel_name, logs.model_name, count(*) as request_count, coalesce(sum(logs.prompt_tokens), 0) as prompt_tokens, coalesce(sum(logs.completion_tokens), 0) as completion_tokens, coalesce(sum(logs.prompt_tokens), 0) + coalesce(sum(logs.completion_tokens), 0) as total_tokens, coalesce(sum(logs.quota), 0) as quota, coalesce(max(logs.created_at), 0) as last_called_at", dayExpr)
	err := buildUserTokenUsageBaseQuery(query).
		Select(selectClause).
		Group(groupClause).
		Order(resolveUserTokenDailyUsageOrder(query)).
		Limit(num).
		Offset(startIdx).
		Scan(&summaries).Error
	return summaries, total, err
}

func userTokenUsageDayExpr() string {
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		return "intDiv(logs.created_at, 86400) * 86400"
	}
	if common.UsingLogDatabase(common.DatabaseTypeMySQL) {
		return "FLOOR(logs.created_at / 86400) * 86400"
	}
	return "(logs.created_at / 86400) * 86400"
}

func userTokenUsageNeedsMainDBEnrichment() bool {
	return DB != nil && LOG_DB != nil && LOG_DB != DB
}

func getUserTokenUsageSummaryFromSeparatedLogDB(query UserTokenUsageQuery, startIdx int, num int) ([]*UserTokenUsageSummary, int64, error) {
	groupClause := "logs.user_id, logs.token_id, logs.token_name, logs.channel_id, logs.model_name"
	baseQuery, err := buildUserTokenUsageLogBaseQuery(query)
	if err != nil {
		return nil, 0, err
	}
	var groups []struct {
		UserId    int
		TokenId   int
		TokenName string
		ChannelId int
		ModelName string
	}
	if err := baseQuery.Select("logs.user_id, logs.token_id, logs.token_name, logs.channel_id, logs.model_name").Group(groupClause).Scan(&groups).Error; err != nil {
		return nil, 0, err
	}

	total := int64(len(groups))
	var summaries []*UserTokenUsageSummary
	selectClause := "logs.user_id, MAX(logs.username) AS username, logs.token_id, logs.token_name, logs.channel_id, logs.model_name, count(*) as request_count, coalesce(sum(logs.prompt_tokens), 0) as prompt_tokens, coalesce(sum(logs.completion_tokens), 0) as completion_tokens, coalesce(sum(logs.prompt_tokens), 0) + coalesce(sum(logs.completion_tokens), 0) as total_tokens, coalesce(sum(logs.quota), 0) as quota, coalesce(max(logs.created_at), 0) as last_called_at"
	queryDB, err := buildUserTokenUsageLogBaseQuery(query)
	if err != nil {
		return nil, 0, err
	}
	err = queryDB.
		Select(selectClause).
		Group(groupClause).
		Order(resolveUserTokenUsageOrder(query)).
		Limit(num).
		Offset(startIdx).
		Scan(&summaries).Error
	if err != nil {
		return nil, 0, err
	}
	return summaries, total, enrichUserTokenUsageSummaries(summaries)
}

func getUserTokenUsageByDayFromSeparatedLogDB(query UserTokenUsageQuery, startIdx int, num int) ([]*UserTokenDailyUsage, int64, error) {
	dayExpr := userTokenUsageDayExpr()
	groupClause := fmt.Sprintf("%s, logs.user_id, logs.token_id, logs.token_name, logs.channel_id, logs.model_name", dayExpr)
	baseQuery, err := buildUserTokenUsageLogBaseQuery(query)
	if err != nil {
		return nil, 0, err
	}
	var groups []struct {
		Day       int64
		UserId    int
		TokenId   int
		TokenName string
		ChannelId int
		ModelName string
	}
	if err := baseQuery.Select(fmt.Sprintf("%s as day, logs.user_id, logs.token_id, logs.token_name, logs.channel_id, logs.model_name", dayExpr)).Group(groupClause).Scan(&groups).Error; err != nil {
		return nil, 0, err
	}

	total := int64(len(groups))
	var summaries []*UserTokenDailyUsage
	selectClause := fmt.Sprintf("%s as day, logs.user_id, MAX(logs.username) AS username, logs.token_id, logs.token_name, logs.channel_id, logs.model_name, count(*) as request_count, coalesce(sum(logs.prompt_tokens), 0) as prompt_tokens, coalesce(sum(logs.completion_tokens), 0) as completion_tokens, coalesce(sum(logs.prompt_tokens), 0) + coalesce(sum(logs.completion_tokens), 0) as total_tokens, coalesce(sum(logs.quota), 0) as quota, coalesce(max(logs.created_at), 0) as last_called_at", dayExpr)
	queryDB, err := buildUserTokenUsageLogBaseQuery(query)
	if err != nil {
		return nil, 0, err
	}
	err = queryDB.
		Select(selectClause).
		Group(groupClause).
		Order(resolveUserTokenDailyUsageOrder(query)).
		Limit(num).
		Offset(startIdx).
		Scan(&summaries).Error
	if err != nil {
		return nil, 0, err
	}
	return summaries, total, enrichUserTokenDailyUsageSummaries(summaries)
}

func buildUserTokenUsageLogBaseQuery(query UserTokenUsageQuery) (*gorm.DB, error) {
	dbQuery := LOG_DB.Table("logs").Where("logs.type = ?", LogTypeConsume)
	if query.StartTimestamp > 0 {
		dbQuery = dbQuery.Where("logs.created_at >= ?", query.StartTimestamp)
	}
	if query.EndTimestamp > 0 {
		dbQuery = dbQuery.Where("logs.created_at <= ?", query.EndTimestamp)
	}
	if query.UserId > 0 {
		dbQuery = dbQuery.Where("logs.user_id = ?", query.UserId)
	}
	var err error
	if dbQuery, err = applyLogSearchFilter(dbQuery, "logs.username", query.Username); err != nil {
		return nil, err
	}
	if dbQuery, err = applyLogSearchFilter(dbQuery, "logs.token_name", query.TokenName); err != nil {
		return nil, err
	}
	if strings.TrimSpace(query.AuthIndex) != "" {
		dbQuery = dbQuery.Where("logs.token_name = ?", strings.TrimSpace(query.AuthIndex))
	}
	if query.ChannelId > 0 {
		dbQuery = dbQuery.Where("logs.channel_id = ?", query.ChannelId)
	}
	if dbQuery, err = applyLogSearchFilter(dbQuery, "logs.model_name", query.ModelName); err != nil {
		return nil, err
	}
	return dbQuery, nil
}

func buildUserTokenUsageBaseQuery(query UserTokenUsageQuery) *gorm.DB {
	dbQuery := LOG_DB.Table("logs").Joins("LEFT JOIN cliproxy_auth_file_bindings ON cliproxy_auth_file_bindings.user_id = logs.user_id AND cliproxy_auth_file_bindings.auth_index = logs.token_name").Joins("LEFT JOIN channels ON channels.id = logs.channel_id").Joins("LEFT JOIN users ON users.id = logs.user_id").Where("logs.type = ?", LogTypeConsume)
	if query.StartTimestamp > 0 {
		dbQuery = dbQuery.Where("logs.created_at >= ?", query.StartTimestamp)
	}
	if query.EndTimestamp > 0 {
		dbQuery = dbQuery.Where("logs.created_at <= ?", query.EndTimestamp)
	}
	if query.UserId > 0 {
		dbQuery = dbQuery.Where("logs.user_id = ?", query.UserId)
	}
	if strings.TrimSpace(query.Username) != "" {
		dbQuery = dbQuery.Where("logs.username LIKE ?", "%"+strings.TrimSpace(query.Username)+"%")
	}
	if strings.TrimSpace(query.TokenName) != "" {
		dbQuery = dbQuery.Where("logs.token_name LIKE ?", "%"+strings.TrimSpace(query.TokenName)+"%")
	}
	if strings.TrimSpace(query.AuthIndex) != "" {
		dbQuery = dbQuery.Where("cliproxy_auth_file_bindings.auth_index = ?", strings.TrimSpace(query.AuthIndex))
	}
	if query.ChannelId > 0 {
		dbQuery = dbQuery.Where("logs.channel_id = ?", query.ChannelId)
	}
	if strings.TrimSpace(query.ModelName) != "" {
		dbQuery = dbQuery.Where("logs.model_name LIKE ?", "%"+strings.TrimSpace(query.ModelName)+"%")
	}
	return dbQuery
}

func enrichUserTokenUsageSummaries(rows []*UserTokenUsageSummary) error {
	userRemarks, channelNames, bindingNames, err := loadUserTokenUsageEnrichment(rows, nil)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row == nil {
			continue
		}
		row.Remark = userRemarks[row.UserId]
		row.ChannelName = channelNames[row.ChannelId]
		if authName, ok := bindingNames[userTokenUsageBindingKey(row.UserId, row.TokenName)]; ok {
			row.AuthIndex = row.TokenName
			row.AuthName = authName
		}
	}
	return nil
}

func enrichUserTokenDailyUsageSummaries(rows []*UserTokenDailyUsage) error {
	userRemarks, channelNames, _, err := loadUserTokenUsageEnrichment(nil, rows)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row == nil {
			continue
		}
		row.Remark = userRemarks[row.UserId]
		row.ChannelName = channelNames[row.ChannelId]
	}
	return nil
}

func loadUserTokenUsageEnrichment(summaryRows []*UserTokenUsageSummary, dailyRows []*UserTokenDailyUsage) (map[int]string, map[int]string, map[string]string, error) {
	userIDSet := make(map[int]struct{})
	channelIDSet := make(map[int]struct{})
	tokenNameSet := make(map[string]struct{})

	for _, row := range summaryRows {
		if row == nil {
			continue
		}
		if row.UserId > 0 {
			userIDSet[row.UserId] = struct{}{}
		}
		if row.ChannelId > 0 {
			channelIDSet[row.ChannelId] = struct{}{}
		}
		if strings.TrimSpace(row.TokenName) != "" {
			tokenNameSet[row.TokenName] = struct{}{}
		}
	}
	for _, row := range dailyRows {
		if row == nil {
			continue
		}
		if row.UserId > 0 {
			userIDSet[row.UserId] = struct{}{}
		}
		if row.ChannelId > 0 {
			channelIDSet[row.ChannelId] = struct{}{}
		}
		if strings.TrimSpace(row.TokenName) != "" {
			tokenNameSet[row.TokenName] = struct{}{}
		}
	}

	userIDs := intSetToSlice(userIDSet)
	channelIDs := intSetToSlice(channelIDSet)
	tokenNames := stringSetToSlice(tokenNameSet)

	userRemarks := make(map[int]string, len(userIDs))
	if len(userIDs) > 0 {
		var users []struct {
			Id     int    `gorm:"column:id"`
			Remark string `gorm:"column:remark"`
		}
		if err := DB.Unscoped().Table("users").Select("id, remark").Where("id IN ?", userIDs).Find(&users).Error; err != nil {
			return nil, nil, nil, err
		}
		for _, user := range users {
			userRemarks[user.Id] = user.Remark
		}
	}

	channelNames := make(map[int]string, len(channelIDs))
	if len(channelIDs) > 0 {
		var channels []struct {
			Id   int    `gorm:"column:id"`
			Name string `gorm:"column:name"`
		}
		if err := DB.Table("channels").Select("id, name").Where("id IN ?", channelIDs).Find(&channels).Error; err != nil {
			return nil, nil, nil, err
		}
		for _, channel := range channels {
			channelNames[channel.Id] = channel.Name
		}
	}

	bindingNames := make(map[string]string)
	if len(userIDs) > 0 && len(tokenNames) > 0 {
		var bindings []struct {
			UserId    int    `gorm:"column:user_id"`
			AuthIndex string `gorm:"column:auth_index"`
			AuthName  string `gorm:"column:auth_name"`
		}
		if err := DB.Table("cliproxy_auth_file_bindings").
			Select("user_id, auth_index, auth_name").
			Where("user_id IN ? AND auth_index IN ?", userIDs, tokenNames).
			Find(&bindings).Error; err != nil {
			return nil, nil, nil, err
		}
		for _, binding := range bindings {
			bindingNames[userTokenUsageBindingKey(binding.UserId, binding.AuthIndex)] = binding.AuthName
		}
	}

	return userRemarks, channelNames, bindingNames, nil
}

func userTokenUsageBindingKey(userID int, authIndex string) string {
	return fmt.Sprintf("%d\x00%s", userID, authIndex)
}

func intSetToSlice(set map[int]struct{}) []int {
	values := make([]int, 0, len(set))
	for value := range set {
		values = append(values, value)
	}
	return values
}

func stringSetToSlice(set map[string]struct{}) []string {
	values := make([]string, 0, len(set))
	for value := range set {
		values = append(values, value)
	}
	return values
}

func resolveUserTokenUsageOrder(query UserTokenUsageQuery) string {
	sortOrder := strings.ToLower(strings.TrimSpace(query.SortOrder))
	if sortOrder != "asc" {
		sortOrder = "desc"
	}
	switch strings.TrimSpace(query.SortBy) {
	case "request_count":
		return "request_count " + sortOrder
	case "prompt_tokens":
		return "prompt_tokens " + sortOrder
	case "completion_tokens":
		return "completion_tokens " + sortOrder
	case "total_tokens":
		return "total_tokens " + sortOrder
	case "quota":
		return "quota " + sortOrder
	case "last_called_at":
		return "last_called_at " + sortOrder
	default:
		return "last_called_at desc"
	}
}

func resolveUserTokenDailyUsageOrder(query UserTokenUsageQuery) string {
	if strings.TrimSpace(query.SortBy) == "day" {
		sortOrder := strings.ToLower(strings.TrimSpace(query.SortOrder))
		if sortOrder != "asc" {
			sortOrder = "desc"
		}
		return "day " + sortOrder + ", last_called_at desc"
	}
	if strings.TrimSpace(query.SortBy) != "" {
		return resolveUserTokenUsageOrder(query)
	}
	return "day desc, last_called_at desc"
}

func ValidateCliproxyAuthFileBindingUpdate(update CliproxyAuthFileBindingUpdate) error {
	if update.UserId == 0 {
		return errors.New("用户不能为空")
	}
	if strings.TrimSpace(update.AuthIndex) == "" {
		return errors.New("认证文件索引不能为空")
	}
	return nil
}

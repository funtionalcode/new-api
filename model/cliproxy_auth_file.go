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
	Description              string `json:"description" gorm:"type:text"`
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
	Username  string
	AuthIndex string
	Enabled   *bool
}

type CliproxyAuthFileBindingUpdate struct {
	UserId      int
	Username    string
	AuthIndex   string
	AuthName    string
	AuthFile    string
	Description string
	AccountId   string
	Enabled     bool
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
	TokenId          int    `json:"token_id"`
	TokenName        string `json:"token_name"`
	AuthIndex        string `json:"auth_index"`
	AuthName         string `json:"auth_name"`
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
	TokenId          int    `json:"token_id"`
	TokenName        string `json:"token_name"`
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
	err := dbQuery.Order("id desc").Limit(num).Offset(startIdx).Find(&bindings).Error
	return bindings, total, err
}

func buildCliproxyAuthFileBindingQuery(query CliproxyAuthFileBindingQuery) *gorm.DB {
	dbQuery := DB.Model(&CliproxyAuthFileBinding{})
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
		Description:              update.Description,
		AccountId:                update.AccountId,
		Enabled:                  update.Enabled,
		LastRefreshedAt:          binding.LastRefreshedAt,
		LastUsageTokens:          binding.LastUsageTokens,
		LastUsageQuota:           binding.LastUsageQuota,
		LastPlanType:             binding.LastPlanType,
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
		Description:              binding.Description,
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
	baseQuery := buildUserTokenUsageBaseQuery(query)
	var groups []struct {
		UserId    int
		TokenId   int
		TokenName string
	}
	if err := baseQuery.Select("logs.user_id, logs.token_id, logs.token_name").Group("logs.user_id, logs.token_id, logs.token_name").Scan(&groups).Error; err != nil {
		return nil, 0, err
	}
	total := int64(len(groups))
	var summaries []*UserTokenUsageSummary
	selectClause := "logs.user_id, MAX(logs.username) AS username, logs.token_id, logs.token_name, coalesce(max(cliproxy_auth_file_bindings.auth_index), '') as auth_index, coalesce(max(cliproxy_auth_file_bindings.auth_name), '') as auth_name, count(*) as request_count, coalesce(sum(logs.prompt_tokens), 0) as prompt_tokens, coalesce(sum(logs.completion_tokens), 0) as completion_tokens, coalesce(sum(logs.prompt_tokens), 0) + coalesce(sum(logs.completion_tokens), 0) as total_tokens, coalesce(sum(logs.quota), 0) as quota, coalesce(max(logs.created_at), 0) as last_called_at"
	err := buildUserTokenUsageBaseQuery(query).
		Select(selectClause).
		Group("logs.user_id, logs.token_id, logs.token_name").
		Order(resolveUserTokenUsageOrder(query)).
		Limit(num).
		Offset(startIdx).
		Scan(&summaries).Error
	return summaries, total, err
}

func GetUserTokenUsageByDay(query UserTokenUsageQuery, startIdx int, num int) ([]*UserTokenDailyUsage, int64, error) {
	dayExpr := userTokenUsageDayExpr()
	baseQuery := buildUserTokenUsageBaseQuery(query)
	var groups []struct {
		Day       int64
		UserId    int
		TokenId   int
		TokenName string
	}
	groupClause := fmt.Sprintf("%s, logs.user_id, logs.token_id, logs.token_name", dayExpr)
	if err := baseQuery.Select(fmt.Sprintf("%s as day, logs.user_id, logs.token_id, logs.token_name", dayExpr)).Group(groupClause).Scan(&groups).Error; err != nil {
		return nil, 0, err
	}
	total := int64(len(groups))
	var summaries []*UserTokenDailyUsage
	selectClause := fmt.Sprintf("%s as day, logs.user_id, MAX(logs.username) AS username, logs.token_id, logs.token_name, count(*) as request_count, coalesce(sum(logs.prompt_tokens), 0) as prompt_tokens, coalesce(sum(logs.completion_tokens), 0) as completion_tokens, coalesce(sum(logs.prompt_tokens), 0) + coalesce(sum(logs.completion_tokens), 0) as total_tokens, coalesce(sum(logs.quota), 0) as quota, coalesce(max(logs.created_at), 0) as last_called_at", dayExpr)
	err := buildUserTokenUsageBaseQuery(query).
		Select(selectClause).
		Group(fmt.Sprintf("%s, logs.user_id, logs.token_id, logs.token_name", dayExpr)).
		Order(resolveUserTokenDailyUsageOrder(query)).
		Limit(num).
		Offset(startIdx).
		Scan(&summaries).Error
	return summaries, total, err
}

func userTokenUsageDayExpr() string {
	if common.UsingMySQL {
		return "FLOOR(logs.created_at / 86400) * 86400"
	}
	return "(logs.created_at / 86400) * 86400"
}

func buildUserTokenUsageBaseQuery(query UserTokenUsageQuery) *gorm.DB {
	dbQuery := LOG_DB.Table("logs").Joins("LEFT JOIN cliproxy_auth_file_bindings ON cliproxy_auth_file_bindings.user_id = logs.user_id").Where("logs.type = ?", LogTypeConsume)
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
	return dbQuery
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

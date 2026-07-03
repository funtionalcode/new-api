package model

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

type GLMQuotaBinding struct {
	Id                     int    `json:"id" gorm:"primaryKey"`
	Name                   string `json:"name" gorm:"size:128;index;not null"`
	Note                   string `json:"note" gorm:"type:text"`
	RequestCurl            string `json:"-" gorm:"type:text;not null"`
	PlanType               string `json:"plan_type" gorm:"size:64;default:''"`
	FiveHourLimitTokens    int64  `json:"five_hour_limit_tokens" gorm:"bigint;default:0"`
	WeeklyLimitTokens      int64  `json:"weekly_limit_tokens" gorm:"bigint;default:0"`
	LastFiveHourUsedTokens int64  `json:"last_five_hour_used_tokens" gorm:"bigint;default:0"`
	LastWeeklyUsedTokens   int64  `json:"last_weekly_used_tokens" gorm:"bigint;default:0"`
	LastFiveHourPercent    int    `json:"last_five_hour_percent" gorm:"default:0"`
	LastWeeklyPercent      int    `json:"last_weekly_percent" gorm:"default:0"`
	LastModelCallCount     int64  `json:"last_model_call_count" gorm:"bigint;default:0"`
	LastModelSummary       string `json:"last_model_summary" gorm:"type:text"`
	LastRefreshedAt        int64  `json:"last_refreshed_at" gorm:"bigint;default:0"`
	LastError              string `json:"last_error" gorm:"type:text"`
	Enabled                bool   `json:"enabled" gorm:"default:true"`
	CreatedAt              int64  `json:"created_at" gorm:"bigint;index"`
	UpdatedAt              int64  `json:"updated_at" gorm:"bigint"`
	HasCurl                bool   `json:"has_curl" gorm:"-"`
}

type GLMQuotaBindingQuery struct {
	Keyword string
	Enabled *bool
}

type GLMQuotaBindingUpdate struct {
	Name                string
	Note                string
	RequestCurl         string
	UpdateCurl          bool
	PlanType            string
	FiveHourLimitTokens int64
	WeeklyLimitTokens   int64
	Enabled             bool
}

type GLMQuotaUsageRefreshUpdate struct {
	LastFiveHourUsedTokens int64
	LastWeeklyUsedTokens   int64
	LastFiveHourPercent    int
	LastWeeklyPercent      int
	LastModelCallCount     int64
	LastModelSummary       string
	LastError              string
}

func (GLMQuotaBinding) TableName() string {
	return "glm_quota_bindings"
}

func (binding *GLMQuotaBinding) BeforeCreate() error {
	now := time.Now().Unix()
	binding.CreatedAt = now
	binding.UpdatedAt = now
	return nil
}

func (binding *GLMQuotaBinding) BeforeUpdate() error {
	binding.UpdatedAt = time.Now().Unix()
	return nil
}

func (binding *GLMQuotaBinding) AfterFind(tx *gorm.DB) error {
	binding.HasCurl = strings.TrimSpace(binding.RequestCurl) != ""
	return nil
}

func CreateGLMQuotaBinding(binding *GLMQuotaBinding) error {
	return DB.Create(binding).Error
}

func GetGLMQuotaBindingById(id int) (*GLMQuotaBinding, error) {
	var binding GLMQuotaBinding
	err := DB.First(&binding, "id = ?", id).Error
	return &binding, err
}

func GetGLMQuotaBindings(query GLMQuotaBindingQuery, startIdx int, num int) ([]*GLMQuotaBinding, int64, error) {
	var bindings []*GLMQuotaBinding
	dbQuery := buildGLMQuotaBindingQuery(query)
	var total int64
	if err := dbQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := dbQuery.Order("id DESC").Limit(num).Offset(startIdx).Find(&bindings).Error
	return bindings, total, err
}

func buildGLMQuotaBindingQuery(query GLMQuotaBindingQuery) *gorm.DB {
	dbQuery := DB.Model(&GLMQuotaBinding{})
	keyword := strings.TrimSpace(query.Keyword)
	if keyword != "" {
		dbQuery = dbQuery.Where("name LIKE ? OR note LIKE ? OR plan_type LIKE ?", "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}
	if query.Enabled != nil {
		dbQuery = dbQuery.Where("enabled = ?", *query.Enabled)
	}
	return dbQuery
}

func UpdateGLMQuotaBinding(id int, update GLMQuotaBindingUpdate) (*GLMQuotaBinding, error) {
	updates := map[string]any{
		"name":                   strings.TrimSpace(update.Name),
		"note":                   strings.TrimSpace(update.Note),
		"plan_type":              strings.TrimSpace(update.PlanType),
		"five_hour_limit_tokens": update.FiveHourLimitTokens,
		"weekly_limit_tokens":    update.WeeklyLimitTokens,
		"enabled":                update.Enabled,
		"updated_at":             time.Now().Unix(),
	}
	if update.UpdateCurl {
		updates["request_curl"] = strings.TrimSpace(update.RequestCurl)
	}
	if err := DB.Model(&GLMQuotaBinding{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}
	return GetGLMQuotaBindingById(id)
}

func UpdateGLMQuotaBindingUsage(id int, update GLMQuotaUsageRefreshUpdate) (*GLMQuotaBinding, error) {
	updates := map[string]any{
		"last_refreshed_at": time.Now().Unix(),
		"last_error":        strings.TrimSpace(update.LastError),
		"updated_at":        time.Now().Unix(),
	}
	if strings.TrimSpace(update.LastError) == "" {
		updates["last_five_hour_used_tokens"] = update.LastFiveHourUsedTokens
		updates["last_weekly_used_tokens"] = update.LastWeeklyUsedTokens
		updates["last_five_hour_percent"] = update.LastFiveHourPercent
		updates["last_weekly_percent"] = update.LastWeeklyPercent
		updates["last_model_call_count"] = update.LastModelCallCount
		updates["last_model_summary"] = update.LastModelSummary
	}
	if err := DB.Model(&GLMQuotaBinding{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}
	return GetGLMQuotaBindingById(id)
}

func DeleteGLMQuotaBindingById(id int) error {
	return DB.Delete(&GLMQuotaBinding{}, "id = ?", id).Error
}

func ValidateGLMQuotaBindingUpdate(update GLMQuotaBindingUpdate, requireCurl bool) error {
	if strings.TrimSpace(update.Name) == "" {
		return errors.New("名称不能为空")
	}
	if requireCurl && strings.TrimSpace(update.RequestCurl) == "" {
		return errors.New("GLM 额度 curl 不能为空")
	}
	if strings.TrimSpace(update.PlanType) == "" {
		return errors.New("套餐规格不能为空")
	}
	if update.FiveHourLimitTokens < 0 || update.WeeklyLimitTokens < 0 {
		return errors.New("额度不能小于 0")
	}
	if update.FiveHourLimitTokens == 0 || update.WeeklyLimitTokens == 0 {
		return errors.New("套餐规格额度不能为空")
	}
	return nil
}

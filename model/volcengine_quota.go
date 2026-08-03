package model

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

type VolcengineQuotaBinding struct {
	Id                      int     `json:"id" gorm:"primaryKey"`
	Name                    string  `json:"name" gorm:"size:128;index;not null"`
	Note                    string  `json:"note" gorm:"type:text"`
	RequestCurl             string  `json:"request_curl,omitempty" gorm:"type:text;not null"`
	Proxy                   string  `json:"proxy,omitempty" gorm:"type:text"`
	LastPlanType            string  `json:"last_plan_type" gorm:"size:32"`
	LastFiveHourQuota       int64   `json:"last_five_hour_quota" gorm:"bigint;default:0"`
	LastFiveHourUsedAFP     float64 `json:"last_five_hour_used_afp" gorm:"default:0"`
	LastFiveHourSubscribeAt int64   `json:"last_five_hour_subscribe_at" gorm:"bigint;default:0"`
	LastFiveHourResetAt     int64   `json:"last_five_hour_reset_at" gorm:"bigint;default:0"`
	LastDailyQuota          int64   `json:"last_daily_quota" gorm:"bigint;default:0"`
	LastDailyUsedAFP        float64 `json:"last_daily_used_afp" gorm:"default:0"`
	LastDailySubscribeAt    int64   `json:"last_daily_subscribe_at" gorm:"bigint;default:0"`
	LastDailyResetAt        int64   `json:"last_daily_reset_at" gorm:"bigint;default:0"`
	LastWeeklyQuota         int64   `json:"last_weekly_quota" gorm:"bigint;default:0"`
	LastWeeklyUsedAFP       float64 `json:"last_weekly_used_afp" gorm:"default:0"`
	LastWeeklySubscribeAt   int64   `json:"last_weekly_subscribe_at" gorm:"bigint;default:0"`
	LastWeeklyResetAt       int64   `json:"last_weekly_reset_at" gorm:"bigint;default:0"`
	LastMonthlyQuota        int64   `json:"last_monthly_quota" gorm:"bigint;default:0"`
	LastMonthlyUsedAFP      float64 `json:"last_monthly_used_afp" gorm:"default:0"`
	LastMonthlySubscribeAt  int64   `json:"last_monthly_subscribe_at" gorm:"bigint;default:0"`
	LastMonthlyResetAt      int64   `json:"last_monthly_reset_at" gorm:"bigint;default:0"`
	LastRefreshedAt         int64   `json:"last_refreshed_at" gorm:"bigint;default:0"`
	LastError               string  `json:"last_error" gorm:"type:text"`
	Enabled                 bool    `json:"enabled"`
	CreatedAt               int64   `json:"created_at" gorm:"bigint;index"`
	UpdatedAt               int64   `json:"updated_at" gorm:"bigint"`
	HasCurl                 bool    `json:"has_curl" gorm:"-"`
}

type VolcengineQuotaBindingQuery struct {
	Keyword string
	Enabled *bool
}

type VolcengineQuotaBindingUpdate struct {
	Name        string
	Note        string
	RequestCurl string
	UpdateCurl  bool
	Proxy       string
	UpdateProxy bool
	Enabled     bool
}

type VolcengineQuotaUsageRefreshUpdate struct {
	LastPlanType            string
	LastFiveHourQuota       int64
	LastFiveHourUsedAFP     float64
	LastFiveHourSubscribeAt int64
	LastFiveHourResetAt     int64
	LastDailyQuota          int64
	LastDailyUsedAFP        float64
	LastDailySubscribeAt    int64
	LastDailyResetAt        int64
	LastWeeklyQuota         int64
	LastWeeklyUsedAFP       float64
	LastWeeklySubscribeAt   int64
	LastWeeklyResetAt       int64
	LastMonthlyQuota        int64
	LastMonthlyUsedAFP      float64
	LastMonthlySubscribeAt  int64
	LastMonthlyResetAt      int64
	LastError               string
}

func (VolcengineQuotaBinding) TableName() string {
	return "volcengine_quota_bindings"
}

func (binding *VolcengineQuotaBinding) BeforeCreate() error {
	now := time.Now().Unix()
	binding.CreatedAt = now
	binding.UpdatedAt = now
	return nil
}

func (binding *VolcengineQuotaBinding) BeforeUpdate() error {
	binding.UpdatedAt = time.Now().Unix()
	return nil
}

func (binding *VolcengineQuotaBinding) AfterFind(tx *gorm.DB) error {
	binding.HasCurl = strings.TrimSpace(binding.RequestCurl) != ""
	return nil
}

func CreateVolcengineQuotaBinding(binding *VolcengineQuotaBinding) error {
	return DB.Create(binding).Error
}

func GetVolcengineQuotaBindingById(id int) (*VolcengineQuotaBinding, error) {
	var binding VolcengineQuotaBinding
	err := DB.First(&binding, "id = ?", id).Error
	return &binding, err
}

func GetVolcengineQuotaBindings(query VolcengineQuotaBindingQuery, startIdx int, num int) ([]*VolcengineQuotaBinding, int64, error) {
	var bindings []*VolcengineQuotaBinding
	dbQuery := DB.Model(&VolcengineQuotaBinding{})
	keyword := strings.TrimSpace(query.Keyword)
	if keyword != "" {
		dbQuery = dbQuery.Where("name LIKE ? OR note LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	if query.Enabled != nil {
		dbQuery = dbQuery.Where("enabled = ?", *query.Enabled)
	}
	var total int64
	if err := dbQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := dbQuery.Order("id DESC").Limit(num).Offset(startIdx).Find(&bindings).Error
	return bindings, total, err
}

func UpdateVolcengineQuotaBinding(id int, update VolcengineQuotaBindingUpdate) (*VolcengineQuotaBinding, error) {
	updates := map[string]any{
		"name":       strings.TrimSpace(update.Name),
		"note":       strings.TrimSpace(update.Note),
		"enabled":    update.Enabled,
		"updated_at": time.Now().Unix(),
	}
	if update.UpdateCurl {
		updates["request_curl"] = strings.TrimSpace(update.RequestCurl)
	}
	if update.UpdateProxy {
		updates["proxy"] = strings.TrimSpace(update.Proxy)
	}
	if err := DB.Model(&VolcengineQuotaBinding{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}
	return GetVolcengineQuotaBindingById(id)
}

func UpdateVolcengineQuotaBindingUsage(id int, update VolcengineQuotaUsageRefreshUpdate) (*VolcengineQuotaBinding, error) {
	updates := map[string]any{
		"last_refreshed_at": time.Now().Unix(),
		"last_error":        strings.TrimSpace(update.LastError),
		"updated_at":        time.Now().Unix(),
	}
	if strings.TrimSpace(update.LastError) == "" {
		updates["last_plan_type"] = strings.TrimSpace(update.LastPlanType)
		updates["last_five_hour_quota"] = update.LastFiveHourQuota
		updates["last_five_hour_used_afp"] = update.LastFiveHourUsedAFP
		updates["last_five_hour_subscribe_at"] = update.LastFiveHourSubscribeAt
		updates["last_five_hour_reset_at"] = update.LastFiveHourResetAt
		updates["last_daily_quota"] = update.LastDailyQuota
		updates["last_daily_used_afp"] = update.LastDailyUsedAFP
		updates["last_daily_subscribe_at"] = update.LastDailySubscribeAt
		updates["last_daily_reset_at"] = update.LastDailyResetAt
		updates["last_weekly_quota"] = update.LastWeeklyQuota
		updates["last_weekly_used_afp"] = update.LastWeeklyUsedAFP
		updates["last_weekly_subscribe_at"] = update.LastWeeklySubscribeAt
		updates["last_weekly_reset_at"] = update.LastWeeklyResetAt
		updates["last_monthly_quota"] = update.LastMonthlyQuota
		updates["last_monthly_used_afp"] = update.LastMonthlyUsedAFP
		updates["last_monthly_subscribe_at"] = update.LastMonthlySubscribeAt
		updates["last_monthly_reset_at"] = update.LastMonthlyResetAt
	}
	if err := DB.Model(&VolcengineQuotaBinding{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}
	return GetVolcengineQuotaBindingById(id)
}

func DeleteVolcengineQuotaBindingById(id int) error {
	return DB.Delete(&VolcengineQuotaBinding{}, "id = ?", id).Error
}

func ValidateVolcengineQuotaBindingUpdate(update VolcengineQuotaBindingUpdate, requireCurl bool) error {
	if strings.TrimSpace(update.Name) == "" {
		return errors.New("名称不能为空")
	}
	if requireCurl && strings.TrimSpace(update.RequestCurl) == "" {
		return errors.New("火山额度 curl 不能为空")
	}
	return nil
}

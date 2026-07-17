package model

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

type KimiQuotaBinding struct {
	Id                          int    `json:"id" gorm:"primaryKey"`
	Name                        string `json:"name" gorm:"size:128;index;not null"`
	Note                        string `json:"note" gorm:"type:text"`
	RequestCurl                 string `json:"request_curl,omitempty" gorm:"type:text;not null"`
	Proxy                       string `json:"proxy,omitempty" gorm:"type:text"`
	LastCurrentQuota            int64  `json:"last_current_quota" gorm:"bigint;default:0"`
	LastVoucherCurrentQuota     int64  `json:"last_voucher_current_quota" gorm:"bigint;default:0"`
	LastAccumulatedQuota        int64  `json:"last_accumulated_quota" gorm:"bigint;default:0"`
	LastVoucherAccumulatedQuota int64  `json:"last_voucher_accumulated_quota" gorm:"bigint;default:0"`
	LastVoucherExpiredQuota     int64  `json:"last_voucher_expired_quota" gorm:"bigint;default:0"`
	LastRechargeBonusPercent    int    `json:"last_recharge_bonus_percent" gorm:"default:0"`
	LastUsedQuota               int64  `json:"last_used_quota" gorm:"bigint;default:0"`
	LastRemainingQuota          int64  `json:"last_remaining_quota" gorm:"bigint;default:0"`
	LastTotalQuota              int64  `json:"last_total_quota" gorm:"bigint;default:0"`
	LastRemainingPercent        int    `json:"last_remaining_percent" gorm:"default:0"`
	LastRefreshedAt             int64  `json:"last_refreshed_at" gorm:"bigint;default:0"`
	LastError                   string `json:"last_error" gorm:"type:text"`
	Enabled                     bool   `json:"enabled"`
	CreatedAt                   int64  `json:"created_at" gorm:"bigint;index"`
	UpdatedAt                   int64  `json:"updated_at" gorm:"bigint"`
	HasCurl                     bool   `json:"has_curl" gorm:"-"`
}

type KimiQuotaBindingQuery struct {
	Keyword string
	Enabled *bool
}

type KimiQuotaBindingUpdate struct {
	Name        string
	Note        string
	RequestCurl string
	UpdateCurl  bool
	Proxy       string
	UpdateProxy bool
	Enabled     bool
}

type KimiQuotaUsageRefreshUpdate struct {
	LastCurrentQuota            int64
	LastVoucherCurrentQuota     int64
	LastAccumulatedQuota        int64
	LastVoucherAccumulatedQuota int64
	LastVoucherExpiredQuota     int64
	LastRechargeBonusPercent    int
	LastUsedQuota               int64
	LastRemainingQuota          int64
	LastTotalQuota              int64
	LastRemainingPercent        int
	LastError                   string
}

func (KimiQuotaBinding) TableName() string {
	return "kimi_quota_bindings"
}

func (binding *KimiQuotaBinding) BeforeCreate() error {
	now := time.Now().Unix()
	binding.CreatedAt = now
	binding.UpdatedAt = now
	return nil
}

func (binding *KimiQuotaBinding) BeforeUpdate() error {
	binding.UpdatedAt = time.Now().Unix()
	return nil
}

func (binding *KimiQuotaBinding) AfterFind(tx *gorm.DB) error {
	binding.HasCurl = strings.TrimSpace(binding.RequestCurl) != ""
	return nil
}

func CreateKimiQuotaBinding(binding *KimiQuotaBinding) error {
	return DB.Create(binding).Error
}

func GetKimiQuotaBindingById(id int) (*KimiQuotaBinding, error) {
	var binding KimiQuotaBinding
	err := DB.First(&binding, "id = ?", id).Error
	return &binding, err
}

func GetKimiQuotaBindings(query KimiQuotaBindingQuery, startIdx int, num int) ([]*KimiQuotaBinding, int64, error) {
	var bindings []*KimiQuotaBinding
	dbQuery := buildKimiQuotaBindingQuery(query)
	var total int64
	if err := dbQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := dbQuery.Order("id DESC").Limit(num).Offset(startIdx).Find(&bindings).Error
	return bindings, total, err
}

func buildKimiQuotaBindingQuery(query KimiQuotaBindingQuery) *gorm.DB {
	dbQuery := DB.Model(&KimiQuotaBinding{})
	keyword := strings.TrimSpace(query.Keyword)
	if keyword != "" {
		dbQuery = dbQuery.Where("name LIKE ? OR note LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	if query.Enabled != nil {
		dbQuery = dbQuery.Where("enabled = ?", *query.Enabled)
	}
	return dbQuery
}

func UpdateKimiQuotaBinding(id int, update KimiQuotaBindingUpdate) (*KimiQuotaBinding, error) {
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
	if err := DB.Model(&KimiQuotaBinding{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}
	return GetKimiQuotaBindingById(id)
}

func UpdateKimiQuotaBindingUsage(id int, update KimiQuotaUsageRefreshUpdate) (*KimiQuotaBinding, error) {
	updates := map[string]any{
		"last_refreshed_at": time.Now().Unix(),
		"last_error":        strings.TrimSpace(update.LastError),
		"updated_at":        time.Now().Unix(),
	}
	if strings.TrimSpace(update.LastError) == "" {
		updates["last_current_quota"] = update.LastCurrentQuota
		updates["last_voucher_current_quota"] = update.LastVoucherCurrentQuota
		updates["last_accumulated_quota"] = update.LastAccumulatedQuota
		updates["last_voucher_accumulated_quota"] = update.LastVoucherAccumulatedQuota
		updates["last_voucher_expired_quota"] = update.LastVoucherExpiredQuota
		updates["last_recharge_bonus_percent"] = update.LastRechargeBonusPercent
		updates["last_used_quota"] = update.LastUsedQuota
		updates["last_remaining_quota"] = update.LastRemainingQuota
		updates["last_total_quota"] = update.LastTotalQuota
		updates["last_remaining_percent"] = update.LastRemainingPercent
	}
	if err := DB.Model(&KimiQuotaBinding{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}
	return GetKimiQuotaBindingById(id)
}

func DeleteKimiQuotaBindingById(id int) error {
	return DB.Delete(&KimiQuotaBinding{}, "id = ?", id).Error
}

func ValidateKimiQuotaBindingUpdate(update KimiQuotaBindingUpdate, requireCurl bool) error {
	if strings.TrimSpace(update.Name) == "" {
		return errors.New("名称不能为空")
	}
	if requireCurl && strings.TrimSpace(update.RequestCurl) == "" {
		return errors.New("Kimi 额度 curl 不能为空")
	}
	return nil
}

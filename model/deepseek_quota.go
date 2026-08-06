package model

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

type DeepSeekQuotaBinding struct {
	Id                         int    `json:"id" gorm:"primaryKey"`
	Name                       string `json:"name" gorm:"size:128;index;not null"`
	Note                       string `json:"note" gorm:"type:text"`
	RequestCurl                string `json:"request_curl,omitempty" gorm:"type:text;not null"`
	UsageAmountCurl            string `json:"usage_amount_curl,omitempty" gorm:"type:text"`
	UsageCostCurl              string `json:"usage_cost_curl,omitempty" gorm:"type:text"`
	Proxy                      string `json:"proxy,omitempty" gorm:"type:text"`
	LastMonthlyLimitTokens     int64  `json:"last_monthly_limit_tokens" gorm:"bigint;default:0"`
	LastMonthlyUsedTokens      int64  `json:"last_monthly_used_tokens" gorm:"bigint;default:0"`
	LastMonthlyRemainingTokens int64  `json:"last_monthly_remaining_tokens" gorm:"bigint;default:0"`
	LastMonthlyPercent         int    `json:"last_monthly_percent" gorm:"default:0"`
	LastTotalAvailableTokens   int64  `json:"last_total_available_tokens" gorm:"bigint;default:0"`
	LastTodayUsedTokens        int64  `json:"last_today_used_tokens" gorm:"bigint;default:0"`
	LastNormalWallets          string `json:"last_normal_wallets" gorm:"type:text"`
	LastBonusWallets           string `json:"last_bonus_wallets" gorm:"type:text"`
	LastMonthlyCosts           string `json:"last_monthly_costs" gorm:"type:text"`
	LastTodayCosts             string `json:"last_today_costs" gorm:"type:text"`
	LastTotalCosts             string `json:"last_total_costs" gorm:"type:text"`
	LastRequestCount           int64  `json:"last_request_count" gorm:"bigint;default:0"`
	LastRefreshedAt            int64  `json:"last_refreshed_at" gorm:"bigint;default:0"`
	LastError                  string `json:"last_error" gorm:"type:text"`
	Enabled                    bool   `json:"enabled" gorm:"default:true"`
	CreatedAt                  int64  `json:"created_at" gorm:"bigint;index"`
	UpdatedAt                  int64  `json:"updated_at" gorm:"bigint"`
	HasCurl                    bool   `json:"has_curl" gorm:"-"`
	HasUsageAmountCurl         bool   `json:"has_usage_amount_curl" gorm:"-"`
	HasUsageCostCurl           bool   `json:"has_usage_cost_curl" gorm:"-"`
}

type DeepSeekQuotaBindingQuery struct {
	Keyword string
	Enabled *bool
}

type DeepSeekQuotaBindingUpdate struct {
	Name                  string
	Note                  string
	RequestCurl           string
	UpdateCurl            bool
	UsageAmountCurl       string
	UpdateUsageAmountCurl bool
	UsageCostCurl         string
	UpdateUsageCostCurl   bool
	Proxy                 string
	UpdateProxy           bool
	Enabled               bool
}

type DeepSeekQuotaUsageRefreshUpdate struct {
	LastMonthlyLimitTokens     int64
	LastMonthlyUsedTokens      int64
	LastMonthlyRemainingTokens int64
	LastMonthlyPercent         int
	LastTotalAvailableTokens   int64
	LastTodayUsedTokens        int64
	LastNormalWallets          string
	LastBonusWallets           string
	LastMonthlyCosts           string
	LastTodayCosts             string
	LastTotalCosts             string
	LastRequestCount           int64
	LastError                  string
}

func (DeepSeekQuotaBinding) TableName() string {
	return "deepseek_quota_bindings"
}

func (binding *DeepSeekQuotaBinding) BeforeCreate() error {
	now := time.Now().Unix()
	binding.CreatedAt = now
	binding.UpdatedAt = now
	return nil
}

func (binding *DeepSeekQuotaBinding) BeforeUpdate() error {
	binding.UpdatedAt = time.Now().Unix()
	return nil
}

func (binding *DeepSeekQuotaBinding) AfterFind(tx *gorm.DB) error {
	binding.HasCurl = strings.TrimSpace(binding.RequestCurl) != ""
	binding.HasUsageAmountCurl = strings.TrimSpace(binding.UsageAmountCurl) != ""
	binding.HasUsageCostCurl = strings.TrimSpace(binding.UsageCostCurl) != ""
	return nil
}

func CreateDeepSeekQuotaBinding(binding *DeepSeekQuotaBinding) error {
	return DB.Create(binding).Error
}

func GetDeepSeekQuotaBindingById(id int) (*DeepSeekQuotaBinding, error) {
	var binding DeepSeekQuotaBinding
	err := DB.First(&binding, "id = ?", id).Error
	return &binding, err
}

func GetDeepSeekQuotaBindings(query DeepSeekQuotaBindingQuery, startIdx int, num int) ([]*DeepSeekQuotaBinding, int64, error) {
	var bindings []*DeepSeekQuotaBinding
	dbQuery := buildDeepSeekQuotaBindingQuery(query)
	var total int64
	if err := dbQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := dbQuery.Order("id DESC").Limit(num).Offset(startIdx).Find(&bindings).Error
	return bindings, total, err
}

func buildDeepSeekQuotaBindingQuery(query DeepSeekQuotaBindingQuery) *gorm.DB {
	dbQuery := DB.Model(&DeepSeekQuotaBinding{})
	keyword := strings.TrimSpace(query.Keyword)
	if keyword != "" {
		dbQuery = dbQuery.Where("name LIKE ? OR note LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	if query.Enabled != nil {
		dbQuery = dbQuery.Where("enabled = ?", *query.Enabled)
	}
	return dbQuery
}

func UpdateDeepSeekQuotaBinding(id int, update DeepSeekQuotaBindingUpdate) (*DeepSeekQuotaBinding, error) {
	if update.UpdateUsageAmountCurl || update.UpdateUsageCostCurl {
		binding, err := GetDeepSeekQuotaBindingById(id)
		if err != nil {
			return nil, err
		}
		usageAmountCurl := binding.UsageAmountCurl
		if update.UpdateUsageAmountCurl {
			usageAmountCurl = update.UsageAmountCurl
		}
		usageCostCurl := binding.UsageCostCurl
		if update.UpdateUsageCostCurl {
			usageCostCurl = update.UsageCostCurl
		}
		if (strings.TrimSpace(usageAmountCurl) == "") != (strings.TrimSpace(usageCostCurl) == "") {
			return nil, errors.New("DeepSeek 用量统计和消费统计 curl 必须同时配置")
		}
	}

	updates := map[string]any{
		"name":       strings.TrimSpace(update.Name),
		"note":       strings.TrimSpace(update.Note),
		"enabled":    update.Enabled,
		"updated_at": time.Now().Unix(),
	}
	if update.UpdateCurl {
		updates["request_curl"] = strings.TrimSpace(update.RequestCurl)
	}
	if update.UpdateUsageAmountCurl {
		updates["usage_amount_curl"] = strings.TrimSpace(update.UsageAmountCurl)
	}
	if update.UpdateUsageCostCurl {
		updates["usage_cost_curl"] = strings.TrimSpace(update.UsageCostCurl)
	}
	if update.UpdateProxy {
		updates["proxy"] = strings.TrimSpace(update.Proxy)
	}
	if err := DB.Model(&DeepSeekQuotaBinding{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}
	return GetDeepSeekQuotaBindingById(id)
}

func UpdateDeepSeekQuotaBindingUsage(id int, update DeepSeekQuotaUsageRefreshUpdate) (*DeepSeekQuotaBinding, error) {
	updates := map[string]any{
		"last_refreshed_at": time.Now().Unix(),
		"last_error":        strings.TrimSpace(update.LastError),
		"updated_at":        time.Now().Unix(),
	}
	if strings.TrimSpace(update.LastError) == "" {
		updates["last_monthly_limit_tokens"] = update.LastMonthlyLimitTokens
		updates["last_monthly_used_tokens"] = update.LastMonthlyUsedTokens
		updates["last_monthly_remaining_tokens"] = update.LastMonthlyRemainingTokens
		updates["last_monthly_percent"] = update.LastMonthlyPercent
		updates["last_total_available_tokens"] = update.LastTotalAvailableTokens
		updates["last_today_used_tokens"] = update.LastTodayUsedTokens
		updates["last_normal_wallets"] = update.LastNormalWallets
		updates["last_bonus_wallets"] = update.LastBonusWallets
		updates["last_monthly_costs"] = update.LastMonthlyCosts
		updates["last_today_costs"] = update.LastTodayCosts
		updates["last_total_costs"] = update.LastTotalCosts
		updates["last_request_count"] = update.LastRequestCount
	}
	if err := DB.Model(&DeepSeekQuotaBinding{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}
	return GetDeepSeekQuotaBindingById(id)
}

func DeleteDeepSeekQuotaBindingById(id int) error {
	return DB.Delete(&DeepSeekQuotaBinding{}, "id = ?", id).Error
}

func ValidateDeepSeekQuotaBindingUpdate(update DeepSeekQuotaBindingUpdate, requireCurl bool) error {
	if strings.TrimSpace(update.Name) == "" {
		return errors.New("名称不能为空")
	}
	if requireCurl && strings.TrimSpace(update.RequestCurl) == "" {
		return errors.New("DeepSeek 账户汇总 curl 不能为空")
	}
	if requireCurl && strings.TrimSpace(update.UsageAmountCurl) == "" {
		return errors.New("DeepSeek 用量统计 curl 不能为空")
	}
	if requireCurl && strings.TrimSpace(update.UsageCostCurl) == "" {
		return errors.New("DeepSeek 消费统计 curl 不能为空")
	}
	return nil
}

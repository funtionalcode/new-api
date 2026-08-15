package model

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

type CursorQuotaBinding struct {
	Id                         int     `json:"id" gorm:"primaryKey"`
	Name                       string  `json:"name" gorm:"size:128;index;not null"`
	Note                       string  `json:"note" gorm:"type:text"`
	RequestCurl                string  `json:"request_curl,omitempty" gorm:"type:text;not null"`
	UsageAmountCurl            string  `json:"usage_amount_curl,omitempty" gorm:"type:text;not null"`
	UsageCostCurl              string  `json:"usage_cost_curl,omitempty" gorm:"type:text;not null"`
	Proxy                      string  `json:"proxy,omitempty" gorm:"type:text"`
	LastPlanName               string  `json:"last_plan_name" gorm:"size:64"`
	LastBillingCycleStartAt    int64   `json:"last_billing_cycle_start_at" gorm:"bigint;default:0"`
	LastBillingCycleEndAt      int64   `json:"last_billing_cycle_end_at" gorm:"bigint;default:0"`
	LastPlanUsedCents          float64 `json:"last_plan_used_cents" gorm:"default:0"`
	LastPlanLimitCents         float64 `json:"last_plan_limit_cents" gorm:"default:0"`
	LastPlanRemainingCents     float64 `json:"last_plan_remaining_cents" gorm:"default:0"`
	LastPlanAPIPercent         float64 `json:"last_plan_api_percent" gorm:"default:0"`
	LastPlanTotalPercent       float64 `json:"last_plan_total_percent" gorm:"default:0"`
	LastOnDemandUsedCents      float64 `json:"last_on_demand_used_cents" gorm:"default:0"`
	LastOnDemandLimitCents     float64 `json:"last_on_demand_limit_cents" gorm:"default:0"`
	LastOnDemandRemainingCents float64 `json:"last_on_demand_remaining_cents" gorm:"default:0"`
	LastTotalInputTokens       int64   `json:"last_total_input_tokens" gorm:"bigint;default:0"`
	LastTotalOutputTokens      int64   `json:"last_total_output_tokens" gorm:"bigint;default:0"`
	LastTotalCacheWriteTokens  int64   `json:"last_total_cache_write_tokens" gorm:"bigint;default:0"`
	LastTotalCacheReadTokens   int64   `json:"last_total_cache_read_tokens" gorm:"bigint;default:0"`
	LastTotalCostCents         float64 `json:"last_total_cost_cents" gorm:"default:0"`
	LastModelUsage             string  `json:"last_model_usage" gorm:"type:text"`
	LastRefreshedAt            int64   `json:"last_refreshed_at" gorm:"bigint;default:0"`
	LastError                  string  `json:"last_error" gorm:"type:text"`
	Enabled                    bool    `json:"enabled"`
	CreatedAt                  int64   `json:"created_at" gorm:"bigint;index"`
	UpdatedAt                  int64   `json:"updated_at" gorm:"bigint"`
	HasCurl                    bool    `json:"has_curl" gorm:"-"`
	HasUsageAmountCurl         bool    `json:"has_usage_amount_curl" gorm:"-"`
	HasUsageCostCurl           bool    `json:"has_usage_cost_curl" gorm:"-"`
}

type CursorQuotaBindingQuery struct {
	Keyword string
	Enabled *bool
}

type CursorQuotaBindingUpdate struct {
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

type CursorQuotaUsageRefreshUpdate struct {
	LastPlanName               string
	LastBillingCycleStartAt    int64
	LastBillingCycleEndAt      int64
	LastPlanUsedCents          float64
	LastPlanLimitCents         float64
	LastPlanRemainingCents     float64
	LastPlanAPIPercent         float64
	LastPlanTotalPercent       float64
	LastOnDemandUsedCents      float64
	LastOnDemandLimitCents     float64
	LastOnDemandRemainingCents float64
	LastTotalInputTokens       int64
	LastTotalOutputTokens      int64
	LastTotalCacheWriteTokens  int64
	LastTotalCacheReadTokens   int64
	LastTotalCostCents         float64
	LastModelUsage             string
	LastError                  string
}

func (CursorQuotaBinding) TableName() string {
	return "cursor_quota_bindings"
}

func (binding *CursorQuotaBinding) BeforeCreate() error {
	now := time.Now().Unix()
	binding.CreatedAt = now
	binding.UpdatedAt = now
	return nil
}

func (binding *CursorQuotaBinding) BeforeUpdate() error {
	binding.UpdatedAt = time.Now().Unix()
	return nil
}

func (binding *CursorQuotaBinding) AfterFind(tx *gorm.DB) error {
	binding.HasCurl = strings.TrimSpace(binding.RequestCurl) != ""
	binding.HasUsageAmountCurl = strings.TrimSpace(binding.UsageAmountCurl) != ""
	binding.HasUsageCostCurl = strings.TrimSpace(binding.UsageCostCurl) != ""
	return nil
}

func CreateCursorQuotaBinding(binding *CursorQuotaBinding) error {
	return DB.Create(binding).Error
}

func GetCursorQuotaBindingById(id int) (*CursorQuotaBinding, error) {
	var binding CursorQuotaBinding
	err := DB.First(&binding, "id = ?", id).Error
	return &binding, err
}

func GetCursorQuotaBindings(query CursorQuotaBindingQuery, startIdx int, num int) ([]*CursorQuotaBinding, int64, error) {
	var bindings []*CursorQuotaBinding
	dbQuery := DB.Model(&CursorQuotaBinding{})
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

func UpdateCursorQuotaBinding(id int, update CursorQuotaBindingUpdate) (*CursorQuotaBinding, error) {
	if update.UpdateCurl || update.UpdateUsageAmountCurl || update.UpdateUsageCostCurl {
		binding, err := GetCursorQuotaBindingById(id)
		if err != nil {
			return nil, err
		}
		requestCurl := binding.RequestCurl
		if update.UpdateCurl {
			requestCurl = update.RequestCurl
		}
		usageAmountCurl := binding.UsageAmountCurl
		if update.UpdateUsageAmountCurl {
			usageAmountCurl = update.UsageAmountCurl
		}
		usageCostCurl := binding.UsageCostCurl
		if update.UpdateUsageCostCurl {
			usageCostCurl = update.UsageCostCurl
		}
		if strings.TrimSpace(requestCurl) == "" || strings.TrimSpace(usageAmountCurl) == "" || strings.TrimSpace(usageCostCurl) == "" {
			return nil, errors.New("Cursor 三个额度 curl 必须同时配置")
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
	if err := DB.Model(&CursorQuotaBinding{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}
	return GetCursorQuotaBindingById(id)
}

func UpdateCursorQuotaBindingUsage(id int, update CursorQuotaUsageRefreshUpdate) (*CursorQuotaBinding, error) {
	updates := map[string]any{
		"last_refreshed_at": time.Now().Unix(),
		"last_error":        strings.TrimSpace(update.LastError),
		"updated_at":        time.Now().Unix(),
	}
	if strings.TrimSpace(update.LastError) == "" {
		updates["last_plan_name"] = strings.TrimSpace(update.LastPlanName)
		updates["last_billing_cycle_start_at"] = update.LastBillingCycleStartAt
		updates["last_billing_cycle_end_at"] = update.LastBillingCycleEndAt
		updates["last_plan_used_cents"] = update.LastPlanUsedCents
		updates["last_plan_limit_cents"] = update.LastPlanLimitCents
		updates["last_plan_remaining_cents"] = update.LastPlanRemainingCents
		updates["last_plan_api_percent"] = update.LastPlanAPIPercent
		updates["last_plan_total_percent"] = update.LastPlanTotalPercent
		updates["last_on_demand_used_cents"] = update.LastOnDemandUsedCents
		updates["last_on_demand_limit_cents"] = update.LastOnDemandLimitCents
		updates["last_on_demand_remaining_cents"] = update.LastOnDemandRemainingCents
		updates["last_total_input_tokens"] = update.LastTotalInputTokens
		updates["last_total_output_tokens"] = update.LastTotalOutputTokens
		updates["last_total_cache_write_tokens"] = update.LastTotalCacheWriteTokens
		updates["last_total_cache_read_tokens"] = update.LastTotalCacheReadTokens
		updates["last_total_cost_cents"] = update.LastTotalCostCents
		updates["last_model_usage"] = update.LastModelUsage
	}
	if err := DB.Model(&CursorQuotaBinding{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}
	return GetCursorQuotaBindingById(id)
}

func DeleteCursorQuotaBindingById(id int) error {
	return DB.Delete(&CursorQuotaBinding{}, "id = ?", id).Error
}

func ValidateCursorQuotaBindingUpdate(update CursorQuotaBindingUpdate, requireCurl bool) error {
	if strings.TrimSpace(update.Name) == "" {
		return errors.New("名称不能为空")
	}
	if requireCurl && strings.TrimSpace(update.RequestCurl) == "" {
		return errors.New("Cursor 当前周期用量 curl 不能为空")
	}
	if requireCurl && strings.TrimSpace(update.UsageAmountCurl) == "" {
		return errors.New("Cursor 聚合用量 curl 不能为空")
	}
	if requireCurl && strings.TrimSpace(update.UsageCostCurl) == "" {
		return errors.New("Cursor 套餐信息 curl 不能为空")
	}
	return nil
}

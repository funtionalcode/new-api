package model

import "time"

// TypeSafeUsageBinding 保存控制台请求配置；凭据始终不返回客户端。
type TypeSafeUsageBinding struct {
	Id              int    `json:"id" gorm:"primaryKey"`
	Name            string `json:"name" gorm:"size:128;not null"`
	Note            string `json:"note" gorm:"type:text"`
	RequestCurl     string `json:"-" gorm:"type:text;not null"`
	Proxy           string `json:"-" gorm:"type:text"`
	Enabled         bool   `json:"enabled"`
	LastBuckets     string `json:"last_buckets" gorm:"type:text"`
	LastRefreshedAt int64  `json:"last_refreshed_at"`
	LastError       string `json:"last_error" gorm:"type:text"`
	CreatedAt       int64  `json:"created_at"`
	UpdatedAt       int64  `json:"updated_at"`
	HasCurl         bool   `json:"has_curl" gorm:"-"`
}

func GetTypeSafeUsageBinding(id int) (*TypeSafeUsageBinding, error) {
	var binding TypeSafeUsageBinding
	err := DB.First(&binding, "id = ?", id).Error
	binding.HasCurl = binding.RequestCurl != ""
	return &binding, err
}

func GetTypeSafeUsageBindings(keyword string, offset, limit int) ([]TypeSafeUsageBinding, int64, error) {
	query := DB.Model(&TypeSafeUsageBinding{})
	if keyword != "" {
		query = query.Where("name LIKE ? OR note LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return nil, 0, err
	}
	bindings := []TypeSafeUsageBinding{}
	err := query.Order("id DESC").Offset(offset).Limit(limit).Find(&bindings).Error
	for i := range bindings {
		bindings[i].HasCurl = bindings[i].RequestCurl != ""
	}
	return bindings, count, err
}

func SaveTypeSafeUsageBinding(binding *TypeSafeUsageBinding) error {
	binding.UpdatedAt = time.Now().Unix()
	binding.HasCurl = binding.RequestCurl != ""
	if binding.Id == 0 {
		binding.CreatedAt = binding.UpdatedAt
		return DB.Create(binding).Error
	}
	return DB.Model(&TypeSafeUsageBinding{}).Where("id = ?", binding.Id).Updates(map[string]any{
		"name": binding.Name, "note": binding.Note, "request_curl": binding.RequestCurl,
		"proxy": binding.Proxy, "enabled": binding.Enabled, "updated_at": binding.UpdatedAt,
	}).Error
}

func UpdateTypeSafeUsageSnapshot(id int, buckets, lastError string) error {
	updates := map[string]any{"last_error": lastError}
	if lastError == "" {
		updates["last_buckets"] = buckets
		updates["last_refreshed_at"] = time.Now().Unix()
	}
	return DB.Model(&TypeSafeUsageBinding{}).Where("id = ?", id).Updates(updates).Error
}

func DeleteTypeSafeUsageBinding(id int) error {
	return DB.Delete(&TypeSafeUsageBinding{}, "id = ?", id).Error
}

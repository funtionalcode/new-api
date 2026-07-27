package model

import (
	"fmt"

	"gorm.io/gorm"
)

type ModelStatusLogSummary struct {
	ModelName        string  `json:"model_name" gorm:"column:model_name"`
	RequestCount     int64   `json:"request_count" gorm:"column:request_count"`
	SuccessCount     int64   `json:"success_count" gorm:"column:success_count"`
	ErrorCount       int64   `json:"error_count" gorm:"column:error_count"`
	TokenCount       int64   `json:"token_count" gorm:"column:token_count"`
	Quota            int64   `json:"quota" gorm:"column:quota"`
	AvgUseTime       float64 `json:"avg_use_time" gorm:"column:avg_use_time"`
	TotalUseTime     int64   `json:"total_use_time" gorm:"column:total_use_time"`
	CompletionTokens int64   `json:"completion_tokens" gorm:"column:completion_tokens"`
}

type ModelStatusLogBucket struct {
	ModelName    string `json:"model_name" gorm:"column:model_name"`
	BucketStart  int64  `json:"bucket_start" gorm:"column:bucket_start"`
	RequestCount int64  `json:"request_count" gorm:"column:request_count"`
	SuccessCount int64  `json:"success_count" gorm:"column:success_count"`
	ErrorCount   int64  `json:"error_count" gorm:"column:error_count"`
}

type ModelStatusTodaySummary struct {
	RequestCount int64 `json:"request_count" gorm:"column:request_count"`
	TokenCount   int64 `json:"token_count" gorm:"column:token_count"`
	Quota        int64 `json:"quota" gorm:"column:quota"`
}

type ModelStatusFirstResponseSample struct {
	ModelName string `json:"model_name" gorm:"column:model_name"`
	Other     string `json:"other" gorm:"column:other"`
}

func GetModelStatusLogSummaries(startTime int64, endTime int64, limit int) ([]ModelStatusLogSummary, error) {
	var rows []ModelStatusLogSummary
	query := LOG_DB.Table("logs").
		Select(
			`model_name,
			COUNT(*) AS request_count,
			COALESCE(SUM(CASE WHEN type = ? THEN 1 ELSE 0 END), 0) AS success_count,
			COALESCE(SUM(CASE WHEN type = ? THEN 1 ELSE 0 END), 0) AS error_count,
			COALESCE(SUM(CASE WHEN type = ? THEN prompt_tokens + completion_tokens ELSE 0 END), 0) AS token_count,
			COALESCE(SUM(CASE WHEN type = ? THEN quota ELSE 0 END), 0) AS quota,
			COALESCE(AVG(CASE WHEN type = ? AND use_time > 0 THEN use_time ELSE NULL END), 0) AS avg_use_time,
			COALESCE(SUM(CASE WHEN type = ? AND use_time > 0 THEN use_time ELSE 0 END), 0) AS total_use_time,
			COALESCE(SUM(CASE WHEN type = ? THEN completion_tokens ELSE 0 END), 0) AS completion_tokens`,
			LogTypeConsume,
			LogTypeError,
			LogTypeConsume,
			LogTypeConsume,
			LogTypeConsume,
			LogTypeConsume,
			LogTypeConsume,
		).
		Where("model_name <> ''").
		Where("type IN ?", []int{LogTypeConsume, LogTypeError}).
		Group("model_name").
		Order("request_count DESC")
	query = applyModelStatusTimeRange(query, startTime, endTime)
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Scan(&rows).Error
	return rows, err
}

func GetModelStatusLogBuckets(startTime int64, endTime int64, bucketSeconds int64, modelNames []string) ([]ModelStatusLogBucket, error) {
	if len(modelNames) == 0 {
		return make([]ModelStatusLogBucket, 0), nil
	}
	if bucketSeconds <= 0 {
		bucketSeconds = 3600
	}
	bucketExpr := modelStatusBucketExpr(bucketSeconds)
	var rows []ModelStatusLogBucket
	query := LOG_DB.Table("logs").
		Select(
			fmt.Sprintf(`model_name,
			%s AS bucket_start,
			COUNT(*) AS request_count,
			COALESCE(SUM(CASE WHEN type = ? THEN 1 ELSE 0 END), 0) AS success_count,
			COALESCE(SUM(CASE WHEN type = ? THEN 1 ELSE 0 END), 0) AS error_count`, bucketExpr),
			LogTypeConsume,
			LogTypeError,
		).
		Where("model_name IN ?", modelNames).
		Where("type IN ?", []int{LogTypeConsume, LogTypeError}).
		Group(fmt.Sprintf("model_name, %s", bucketExpr)).
		Order("model_name ASC").
		Order("bucket_start ASC")
	query = applyModelStatusTimeRange(query, startTime, endTime)
	err := query.Scan(&rows).Error
	return rows, err
}

func GetModelStatusTodaySummary(startTime int64, endTime int64) (ModelStatusTodaySummary, error) {
	var row ModelStatusTodaySummary
	query := LOG_DB.Table("logs").
		Select(`COUNT(*) AS request_count,
			COALESCE(SUM(prompt_tokens + completion_tokens), 0) AS token_count,
			COALESCE(SUM(quota), 0) AS quota`).
		Where("model_name <> ''").
		Where("type = ?", LogTypeConsume)
	query = applyModelStatusTimeRange(query, startTime, endTime)
	err := query.Scan(&row).Error
	return row, err
}

func GetModelStatusFirstResponseSamples(startTime int64, endTime int64, modelNames []string, limit int) ([]ModelStatusFirstResponseSample, error) {
	if len(modelNames) == 0 || limit <= 0 {
		return make([]ModelStatusFirstResponseSample, 0), nil
	}
	var rows []ModelStatusFirstResponseSample
	query := LOG_DB.Table("logs").
		Select("model_name, other").
		Where("model_name IN ?", modelNames).
		Where("type = ?", LogTypeConsume).
		Where("other <> ''").
		Order("created_at DESC").
		Limit(limit)
	query = applyModelStatusTimeRange(query, startTime, endTime)
	err := query.Scan(&rows).Error
	return rows, err
}

func modelStatusBucketExpr(bucketSeconds int64) string {
	return fmt.Sprintf("(created_at - (created_at %% %d))", bucketSeconds)
}

func applyModelStatusTimeRange(query *gorm.DB, startTime int64, endTime int64) *gorm.DB {
	if startTime > 0 {
		query = query.Where("created_at >= ?", startTime)
	}
	if endTime > 0 {
		query = query.Where("created_at <= ?", endTime)
	}
	return query
}

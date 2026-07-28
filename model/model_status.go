package model

import (
	"fmt"
	"sort"

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

type ModelStatusErrorSample struct {
	ModelName string `json:"model_name" gorm:"column:model_name"`
	CreatedAt int64  `json:"created_at" gorm:"column:created_at"`
	Content   string `json:"content" gorm:"column:content"`
	Other     string `json:"other" gorm:"column:other"`
	RequestId string `json:"request_id" gorm:"column:request_id"`
}

type modelStatusConsumeSummary struct {
	ModelName        string `gorm:"column:model_name"`
	SuccessCount     int64  `gorm:"column:success_count"`
	PromptTokens     int64  `gorm:"column:prompt_tokens"`
	CompletionTokens int64  `gorm:"column:completion_tokens"`
	Quota            int64  `gorm:"column:quota"`
}

type modelStatusErrorSummary struct {
	ModelName  string `gorm:"column:model_name"`
	ErrorCount int64  `gorm:"column:error_count"`
}

type modelStatusLatencySummary struct {
	ModelName    string `gorm:"column:model_name"`
	TotalUseTime int64  `gorm:"column:total_use_time"`
	UseTimeCount int64  `gorm:"column:use_time_count"`
}

type modelStatusBucketCount struct {
	ModelName   string `gorm:"column:model_name"`
	BucketStart int64  `gorm:"column:bucket_start"`
	Count       int64  `gorm:"column:log_count"`
}

type modelStatusBucketKey struct {
	modelName   string
	bucketStart int64
}

type modelStatusTokenQuotaSummary struct {
	RequestCount     int64 `gorm:"column:request_count"`
	PromptTokens     int64 `gorm:"column:prompt_tokens"`
	CompletionTokens int64 `gorm:"column:completion_tokens"`
	Quota            int64 `gorm:"column:quota"`
}

func GetModelStatusLogSummaries(startTime int64, endTime int64, limit int) ([]ModelStatusLogSummary, error) {
	var rows []ModelStatusLogSummary
	query := LOG_DB.Table("logs").
		Select("model_name, COUNT(*) AS request_count").
		Where("model_name <> ''").
		Where("type IN ?", []int{LogTypeConsume, LogTypeError}).
		Group("model_name").
		Order("request_count DESC")
	query = applyModelStatusTimeRange(query, startTime, endTime)
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Scan(&rows).Error; err != nil || len(rows) == 0 {
		return rows, err
	}

	modelNames := make([]string, 0, len(rows))
	rowIndexByModel := make(map[string]int, len(rows))
	for index, row := range rows {
		modelNames = append(modelNames, row.ModelName)
		rowIndexByModel[row.ModelName] = index
	}

	var consumeRows []modelStatusConsumeSummary
	consumeQuery := LOG_DB.Table("logs").
		Select(`model_name,
			COUNT(*) AS success_count,
			COALESCE(SUM(prompt_tokens), 0) AS prompt_tokens,
			COALESCE(SUM(completion_tokens), 0) AS completion_tokens,
			COALESCE(SUM(quota), 0) AS quota`).
		Where("model_name IN ?", modelNames).
		Where("type = ?", LogTypeConsume).
		Group("model_name")
	consumeQuery = applyModelStatusTimeRange(consumeQuery, startTime, endTime)
	if err := consumeQuery.Scan(&consumeRows).Error; err != nil {
		return nil, err
	}
	for _, consumeRow := range consumeRows {
		index, ok := rowIndexByModel[consumeRow.ModelName]
		if !ok {
			continue
		}
		rows[index].SuccessCount = consumeRow.SuccessCount
		rows[index].TokenCount = consumeRow.PromptTokens + consumeRow.CompletionTokens
		rows[index].Quota = consumeRow.Quota
		rows[index].CompletionTokens = consumeRow.CompletionTokens
	}

	var errorRows []modelStatusErrorSummary
	errorQuery := LOG_DB.Table("logs").
		Select("model_name, COUNT(*) AS error_count").
		Where("model_name IN ?", modelNames).
		Where("type = ?", LogTypeError).
		Group("model_name")
	errorQuery = applyModelStatusTimeRange(errorQuery, startTime, endTime)
	if err := errorQuery.Scan(&errorRows).Error; err != nil {
		return nil, err
	}
	for _, errorRow := range errorRows {
		index, ok := rowIndexByModel[errorRow.ModelName]
		if !ok {
			continue
		}
		rows[index].ErrorCount = errorRow.ErrorCount
	}

	var latencyRows []modelStatusLatencySummary
	latencyQuery := LOG_DB.Table("logs").
		Select(`model_name,
			COALESCE(SUM(use_time), 0) AS total_use_time,
			COUNT(*) AS use_time_count`).
		Where("model_name IN ?", modelNames).
		Where("type = ?", LogTypeConsume).
		Where("use_time > ?", 0).
		Group("model_name")
	latencyQuery = applyModelStatusTimeRange(latencyQuery, startTime, endTime)
	if err := latencyQuery.Scan(&latencyRows).Error; err != nil {
		return nil, err
	}
	for _, latencyRow := range latencyRows {
		index, ok := rowIndexByModel[latencyRow.ModelName]
		if !ok {
			continue
		}
		rows[index].TotalUseTime = latencyRow.TotalUseTime
		if latencyRow.UseTimeCount > 0 {
			rows[index].AvgUseTime = float64(latencyRow.TotalUseTime) / float64(latencyRow.UseTimeCount)
		}
	}

	return rows, nil
}

func GetModelStatusLogBuckets(startTime int64, endTime int64, bucketSeconds int64, modelNames []string) ([]ModelStatusLogBucket, error) {
	if len(modelNames) == 0 {
		return make([]ModelStatusLogBucket, 0), nil
	}
	if bucketSeconds <= 0 {
		bucketSeconds = 3600
	}
	bucketExpr := modelStatusBucketExpr(bucketSeconds)
	bucketRowsByKey := make(map[modelStatusBucketKey]ModelStatusLogBucket)

	var successRows []modelStatusBucketCount
	successQuery := LOG_DB.Table("logs").
		Select(
			fmt.Sprintf(`model_name,
			%s AS bucket_start,
			COUNT(*) AS log_count`, bucketExpr),
		).
		Where("model_name IN ?", modelNames).
		Where("type = ?", LogTypeConsume).
		Group(fmt.Sprintf("model_name, %s", bucketExpr)).
		Order("model_name ASC").
		Order("bucket_start ASC")
	successQuery = applyModelStatusTimeRange(successQuery, startTime, endTime)
	if err := successQuery.Scan(&successRows).Error; err != nil {
		return nil, err
	}
	mergeModelStatusBucketCounts(bucketRowsByKey, successRows, true)

	var errorRows []modelStatusBucketCount
	errorQuery := LOG_DB.Table("logs").
		Select(
			fmt.Sprintf(`model_name,
			%s AS bucket_start,
			COUNT(*) AS log_count`, bucketExpr),
		).
		Where("model_name IN ?", modelNames).
		Where("type = ?", LogTypeError).
		Group(fmt.Sprintf("model_name, %s", bucketExpr)).
		Order("model_name ASC").
		Order("bucket_start ASC")
	errorQuery = applyModelStatusTimeRange(errorQuery, startTime, endTime)
	if err := errorQuery.Scan(&errorRows).Error; err != nil {
		return nil, err
	}
	mergeModelStatusBucketCounts(bucketRowsByKey, errorRows, false)

	rows := make([]ModelStatusLogBucket, 0, len(bucketRowsByKey))
	for _, row := range bucketRowsByKey {
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].ModelName == rows[j].ModelName {
			return rows[i].BucketStart < rows[j].BucketStart
		}
		return rows[i].ModelName < rows[j].ModelName
	})

	return rows, nil
}

func GetModelStatusTodaySummary(startTime int64, endTime int64) (ModelStatusTodaySummary, error) {
	var tokenQuota modelStatusTokenQuotaSummary
	query := LOG_DB.Table("logs").
		Select(`COUNT(*) AS request_count,
			COALESCE(SUM(prompt_tokens), 0) AS prompt_tokens,
			COALESCE(SUM(completion_tokens), 0) AS completion_tokens,
			COALESCE(SUM(quota), 0) AS quota`).
		Where("model_name <> ''").
		Where("type = ?", LogTypeConsume)
	query = applyModelStatusTimeRange(query, startTime, endTime)
	err := query.Scan(&tokenQuota).Error
	return ModelStatusTodaySummary{
		RequestCount: tokenQuota.RequestCount,
		TokenCount:   tokenQuota.PromptTokens + tokenQuota.CompletionTokens,
		Quota:        tokenQuota.Quota,
	}, err
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

func GetModelStatusErrorSamples(startTime int64, endTime int64, modelNames []string, limit int) ([]ModelStatusErrorSample, error) {
	if len(modelNames) == 0 || limit <= 0 {
		return make([]ModelStatusErrorSample, 0), nil
	}
	var rows []ModelStatusErrorSample
	query := LOG_DB.Table("logs").
		Select("model_name, created_at, content, other, request_id").
		Where("model_name IN ?", modelNames).
		Where("type = ?", LogTypeError).
		Order("created_at DESC").
		Limit(limit)
	query = applyModelStatusTimeRange(query, startTime, endTime)
	err := query.Scan(&rows).Error
	return rows, err
}

func modelStatusBucketExpr(bucketSeconds int64) string {
	return fmt.Sprintf("(created_at - (created_at %% %d))", bucketSeconds)
}

func mergeModelStatusBucketCounts(bucketRowsByKey map[modelStatusBucketKey]ModelStatusLogBucket, countRows []modelStatusBucketCount, success bool) {
	for _, countRow := range countRows {
		key := modelStatusBucketKey{modelName: countRow.ModelName, bucketStart: countRow.BucketStart}
		bucketRow := bucketRowsByKey[key]
		bucketRow.ModelName = countRow.ModelName
		bucketRow.BucketStart = countRow.BucketStart
		bucketRow.RequestCount += countRow.Count
		if success {
			bucketRow.SuccessCount += countRow.Count
		} else {
			bucketRow.ErrorCount += countRow.Count
		}
		bucketRowsByKey[key] = bucketRow
	}
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

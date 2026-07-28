package service

import (
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const (
	modelStatusCacheTTL           = time.Minute
	modelStatusWindow             = 24 * time.Hour
	modelStatusBucketSeconds      = int64(3600)
	modelStatusMaxModels          = 200
	modelStatusFirstResponseLimit = 20000
	modelStatusErrorSampleLimit   = 20000
	modelStatusMaxModelErrors     = 20
	modelStatusMaxBucketErrors    = 10
	modelStatusAutoRefreshSeconds = 60
)

type ModelStatusSnapshot struct {
	Overview ModelStatusOverview `json:"overview"`
	Today    ModelStatusToday    `json:"today"`
	Models   []ModelStatusModel  `json:"models"`
}

type ModelStatusOverview struct {
	NormalCount        int     `json:"normal_count"`
	WarningCount       int     `json:"warning_count"`
	ErrorCount         int     `json:"error_count"`
	ModelCount         int     `json:"model_count"`
	RequestCount       int64   `json:"request_count"`
	AvgSuccessRate     float64 `json:"avg_success_rate"`
	UpdatedAt          int64   `json:"updated_at"`
	WindowStart        int64   `json:"window_start"`
	WindowEnd          int64   `json:"window_end"`
	AutoRefreshSeconds int     `json:"auto_refresh_seconds"`
}

type ModelStatusToday struct {
	Tokens       int64   `json:"tokens"`
	Quota        int64   `json:"quota"`
	RequestCount int64   `json:"request_count"`
	Rpm          float64 `json:"rpm"`
	Tpm          float64 `json:"tpm"`
	StartedAt    int64   `json:"started_at"`
}

type ModelStatusModel struct {
	ModelName              string                    `json:"model_name"`
	RequestCount           int64                     `json:"request_count"`
	TokenCount             int64                     `json:"token_count"`
	Quota                  int64                     `json:"quota"`
	SuccessRate            float64                   `json:"success_rate"`
	AvgFirstResponseTimeMs float64                   `json:"avg_first_response_time_ms"`
	AvgLatencyMs           float64                   `json:"avg_latency_ms"`
	Tps                    float64                   `json:"tps"`
	Status                 string                    `json:"status"`
	ErrorDetails           []ModelStatusErrorDetail  `json:"error_details,omitempty"`
	Buckets                []ModelStatusBucketStatus `json:"buckets"`
}

type ModelStatusBucketStatus struct {
	Start        int64                    `json:"start"`
	RequestCount int64                    `json:"request_count"`
	SuccessRate  float64                  `json:"success_rate"`
	Status       string                   `json:"status"`
	ErrorDetails []ModelStatusErrorDetail `json:"error_details,omitempty"`
}

type ModelStatusErrorDetail struct {
	CreatedAt  int64  `json:"created_at"`
	Message    string `json:"message"`
	StatusCode int    `json:"status_code,omitempty"`
	ErrorType  string `json:"error_type,omitempty"`
	ErrorCode  string `json:"error_code,omitempty"`
}

type modelStatusCacheItem struct {
	expiresAt time.Time
	data      *ModelStatusSnapshot
}

type firstResponseAggregate struct {
	sum   float64
	count int64
}

var (
	modelStatusCacheMu sync.Mutex
	modelStatusCache   modelStatusCacheItem
)

func GetModelStatusSnapshot() (*ModelStatusSnapshot, error) {
	now := time.Now()
	modelStatusCacheMu.Lock()
	if modelStatusCache.data != nil && now.Before(modelStatusCache.expiresAt) {
		data := modelStatusCache.data
		modelStatusCacheMu.Unlock()
		return data, nil
	}
	modelStatusCacheMu.Unlock()

	data, err := buildModelStatusSnapshot(now)
	if err != nil {
		return nil, err
	}

	modelStatusCacheMu.Lock()
	modelStatusCache = modelStatusCacheItem{
		expiresAt: now.Add(modelStatusCacheTTL),
		data:      data,
	}
	modelStatusCacheMu.Unlock()

	return data, nil
}

func buildModelStatusSnapshot(now time.Time) (*ModelStatusSnapshot, error) {
	bucketStarts := modelStatusBucketStarts(now)
	windowStart := bucketStarts[0]
	windowEnd := now.Unix()

	summaries, err := model.GetModelStatusLogSummaries(windowStart, windowEnd, modelStatusMaxModels)
	if err != nil {
		return nil, err
	}
	modelNames := make([]string, 0, len(summaries))
	for _, summary := range summaries {
		modelNames = append(modelNames, summary.ModelName)
	}

	buckets, err := model.GetModelStatusLogBuckets(windowStart, windowEnd, modelStatusBucketSeconds, modelNames)
	if err != nil {
		return nil, err
	}
	firstResponseSamples, err := model.GetModelStatusFirstResponseSamples(windowStart, windowEnd, modelNames, modelStatusFirstResponseLimit)
	if err != nil {
		return nil, err
	}
	errorSamples, err := model.GetModelStatusErrorSamples(windowStart, windowEnd, modelNames, modelStatusErrorSampleLimit)
	if err != nil {
		return nil, err
	}

	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()
	todaySummary, err := model.GetModelStatusTodaySummary(todayStart, windowEnd)
	if err != nil {
		return nil, err
	}

	firstResponseMsByModel := modelStatusFirstResponseAverages(firstResponseSamples)
	bucketRowsByModel := modelStatusBucketsByModel(buckets)
	errorDetailsByModel, errorDetailsByBucket := modelStatusErrorDetails(errorSamples)
	modelRows := make([]ModelStatusModel, 0, len(summaries))
	overview := ModelStatusOverview{
		ModelCount:         len(summaries),
		UpdatedAt:          now.Unix(),
		WindowStart:        windowStart,
		WindowEnd:          windowEnd,
		AutoRefreshSeconds: modelStatusAutoRefreshSeconds,
	}

	var successCount int64
	for _, summary := range summaries {
		successRate := modelStatusSuccessRate(summary.SuccessCount, summary.RequestCount)
		status := modelStatusState(summary.RequestCount, successRate)
		switch status {
		case "normal":
			overview.NormalCount++
		case "warning":
			overview.WarningCount++
		case "error":
			overview.ErrorCount++
		}
		overview.RequestCount += summary.RequestCount
		successCount += summary.SuccessCount

		modelRows = append(modelRows, ModelStatusModel{
			ModelName:              summary.ModelName,
			RequestCount:           summary.RequestCount,
			TokenCount:             summary.TokenCount,
			Quota:                  summary.Quota,
			SuccessRate:            successRate,
			AvgFirstResponseTimeMs: firstResponseMsByModel[summary.ModelName],
			AvgLatencyMs:           summary.AvgUseTime * 1000,
			Tps:                    modelStatusTps(summary.CompletionTokens, summary.TotalUseTime),
			Status:                 status,
			ErrorDetails:           errorDetailsByModel[summary.ModelName],
			Buckets:                modelStatusTimeline(bucketStarts, bucketRowsByModel[summary.ModelName], errorDetailsByBucket[summary.ModelName]),
		})
	}
	overview.AvgSuccessRate = modelStatusSuccessRate(successCount, overview.RequestCount)

	todayDurationSeconds := windowEnd - todayStart
	today := ModelStatusToday{
		Tokens:       todaySummary.TokenCount,
		Quota:        todaySummary.Quota,
		RequestCount: todaySummary.RequestCount,
		Rpm:          modelStatusPerMinuteAverage(todaySummary.RequestCount, todayDurationSeconds),
		Tpm:          modelStatusPerMinuteAverage(todaySummary.TokenCount, todayDurationSeconds),
		StartedAt:    todayStart,
	}

	return &ModelStatusSnapshot{
		Overview: overview,
		Today:    today,
		Models:   modelRows,
	}, nil
}

func modelStatusBucketStarts(now time.Time) []int64 {
	lastBucket := now.Truncate(time.Hour)
	firstBucket := lastBucket.Add(-modelStatusWindow + time.Hour)
	buckets := make([]int64, 0, 24)
	for i := 0; i < 24; i++ {
		buckets = append(buckets, firstBucket.Add(time.Duration(i)*time.Hour).Unix())
	}
	return buckets
}

func modelStatusBucketsByModel(rows []model.ModelStatusLogBucket) map[string]map[int64]model.ModelStatusLogBucket {
	result := make(map[string]map[int64]model.ModelStatusLogBucket)
	for _, row := range rows {
		if _, ok := result[row.ModelName]; !ok {
			result[row.ModelName] = make(map[int64]model.ModelStatusLogBucket)
		}
		result[row.ModelName][row.BucketStart] = row
	}
	return result
}

func modelStatusTimeline(bucketStarts []int64, rows map[int64]model.ModelStatusLogBucket, errorDetailsByBucket map[int64][]ModelStatusErrorDetail) []ModelStatusBucketStatus {
	timeline := make([]ModelStatusBucketStatus, 0, len(bucketStarts))
	for _, start := range bucketStarts {
		row := rows[start]
		successRate := modelStatusSuccessRate(row.SuccessCount, row.RequestCount)
		timeline = append(timeline, ModelStatusBucketStatus{
			Start:        start,
			RequestCount: row.RequestCount,
			SuccessRate:  successRate,
			Status:       modelStatusState(row.RequestCount, successRate),
			ErrorDetails: errorDetailsByBucket[start],
		})
	}
	return timeline
}

func modelStatusErrorDetails(samples []model.ModelStatusErrorSample) (map[string][]ModelStatusErrorDetail, map[string]map[int64][]ModelStatusErrorDetail) {
	detailsByModel := make(map[string][]ModelStatusErrorDetail)
	detailsByBucket := make(map[string]map[int64][]ModelStatusErrorDetail)

	for _, sample := range samples {
		if strings.TrimSpace(sample.ModelName) == "" {
			continue
		}
		modelName := sample.ModelName
		detail := modelStatusErrorDetail(sample)
		if len(detailsByModel[modelName]) < modelStatusMaxModelErrors {
			detailsByModel[modelName] = append(detailsByModel[modelName], detail)
		}

		bucketStart := sample.CreatedAt - sample.CreatedAt%modelStatusBucketSeconds
		if _, ok := detailsByBucket[modelName]; !ok {
			detailsByBucket[modelName] = make(map[int64][]ModelStatusErrorDetail)
		}
		if len(detailsByBucket[modelName][bucketStart]) < modelStatusMaxBucketErrors {
			detailsByBucket[modelName][bucketStart] = append(detailsByBucket[modelName][bucketStart], detail)
		}
	}

	return detailsByModel, detailsByBucket
}

func modelStatusErrorDetail(sample model.ModelStatusErrorSample) ModelStatusErrorDetail {
	detail := ModelStatusErrorDetail{
		CreatedAt: sample.CreatedAt,
		Message:   strings.TrimSpace(sample.Content),
	}
	if sample.Other == "" {
		return detail
	}

	var payload map[string]interface{}
	if err := common.UnmarshalJsonStr(sample.Other, &payload); err != nil {
		return detail
	}
	detail.StatusCode = modelStatusStatusCode(payload["status_code"])
	detail.ErrorType = modelStatusString(payload["error_type"])
	detail.ErrorCode = modelStatusString(payload["error_code"])
	return detail
}

func modelStatusSuccessRate(successCount int64, requestCount int64) float64 {
	if requestCount <= 0 {
		return 0
	}
	return float64(successCount) / float64(requestCount)
}

func modelStatusState(requestCount int64, successRate float64) string {
	if requestCount <= 0 {
		return "no_request"
	}
	if successRate >= 0.95 {
		return "normal"
	}
	if successRate >= 0.80 {
		return "warning"
	}
	return "error"
}

func modelStatusTps(completionTokens int64, totalUseTime int64) float64 {
	if completionTokens <= 0 || totalUseTime <= 0 {
		return 0
	}
	return float64(completionTokens) / float64(totalUseTime)
}

func modelStatusPerMinuteAverage(value int64, durationSeconds int64) float64 {
	if value <= 0 {
		return 0
	}
	if durationSeconds <= 0 {
		durationSeconds = 60
	}
	return float64(value) / math.Max(1, float64(durationSeconds)/60)
}

func modelStatusFirstResponseAverages(samples []model.ModelStatusFirstResponseSample) map[string]float64 {
	aggregates := make(map[string]firstResponseAggregate)
	for _, sample := range samples {
		value, ok := modelStatusFirstResponseMs(sample.Other)
		if !ok {
			continue
		}
		aggregate := aggregates[sample.ModelName]
		aggregate.sum += value
		aggregate.count++
		aggregates[sample.ModelName] = aggregate
	}

	result := make(map[string]float64, len(aggregates))
	for modelName, aggregate := range aggregates {
		if aggregate.count > 0 {
			result[modelName] = aggregate.sum / float64(aggregate.count)
		}
	}
	return result
}

func modelStatusFirstResponseMs(other string) (float64, bool) {
	if other == "" {
		return 0, false
	}
	var payload map[string]interface{}
	if err := common.UnmarshalJsonStr(other, &payload); err != nil {
		return 0, false
	}
	value, ok := modelStatusFloat(payload["frt"])
	if !ok || value < 0 {
		return 0, false
	}
	return value, true
}

func modelStatusFloat(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, false
		}
		return v, true
	case float32:
		converted := float64(v)
		if math.IsNaN(converted) || math.IsInf(converted, 0) {
			return 0, false
		}
		return converted, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case string:
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

func modelStatusString(value interface{}) string {
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	return ""
}

func modelStatusStatusCode(value interface{}) int {
	parsed, ok := modelStatusFloat(value)
	if !ok || parsed < 100 || parsed > 999 {
		return 0
	}
	return int(parsed)
}

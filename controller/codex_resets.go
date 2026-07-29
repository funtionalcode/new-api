package controller

import (
	"context"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

const (
	codexResetsTaskDefaultIntervalMinutes = 5
)

// GetCodexResets 返回本地缓存的 Codex 重置事件与统计图表数据。
func GetCodexResets(c *gin.Context) {
	events, err := model.ListCodexResetEvents(200)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	state, err := model.GetOrCreateCodexResetSyncState()
	if err != nil {
		common.ApiError(c, err)
		return
	}

	stats := service.BuildCodexResetsStats(events)
	heatmap := service.BuildCodexResetsHeatmap(events, 26)
	intervals := service.BuildCodexResetsIntervalSeries(events)

	items := make([]gin.H, 0, len(events))
	for _, e := range events {
		if e == nil {
			continue
		}
		items = append(items, gin.H{
			"id":           e.ID,
			"tweet_id":     e.TweetID,
			"tweet_url":    e.TweetURL,
			"text":         e.Text,
			"announced_at": e.AnnouncedAt,
		})
	}

	common.ApiSuccess(c, gin.H{
		"events": items,
		"stats":  stats,
		"charts": gin.H{
			"heatmap":   heatmap,
			"intervals": intervals,
		},
		"sync": gin.H{
			"last_sync_at":      state.LastSyncAt,
			"last_success_at":   state.LastSuccessAt,
			"last_error_at":     state.LastErrorAt,
			"last_error":        state.LastError,
			"last_tweet_id":     state.LastTweetID,
			"last_announced_at": state.LastAnnouncedAt,
			"total_events":      state.TotalEvents,
			"source":            "https://codex-resets.com/",
		},
	})
}

// SyncCodexResets 手动触发一次同步（管理员）。
func SyncCodexResets(c *gin.Context) {
	result, err := service.SyncCodexResets(c.Request.Context(), true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

type codexResetsSyncHandler struct{}

func (codexResetsSyncHandler) Type() string { return model.SystemTaskTypeCodexResetsSync }

func (codexResetsSyncHandler) Enabled() bool {
	return common.GetEnvOrDefaultBool("CODEX_RESETS_SYNC_ENABLED", true)
}

func (codexResetsSyncHandler) Interval() time.Duration {
	minutes := common.GetEnvOrDefault("CODEX_RESETS_SYNC_INTERVAL_MINUTES", codexResetsTaskDefaultIntervalMinutes)
	if minutes < 1 {
		minutes = codexResetsTaskDefaultIntervalMinutes
	}
	return time.Duration(minutes) * time.Minute
}

func (codexResetsSyncHandler) NewPayload() any { return nil }

func (codexResetsSyncHandler) Run(ctx context.Context, task *model.SystemTask, runnerID string) {
	if ctx == nil {
		ctx = context.Background()
	}
	result, err := service.SyncCodexResets(ctx, true)
	if err != nil {
		finishSystemTaskHandler(task, runnerID, model.SystemTaskStatusFailed, nil, err)
		return
	}
	finishSystemTaskHandler(task, runnerID, model.SystemTaskStatusSucceeded, result, nil)
}

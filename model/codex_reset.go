package model

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

// CodexResetEvent 缓存来自 codex-resets.com 的单次重置公告。
type CodexResetEvent struct {
	ID          int64  `json:"id" gorm:"primaryKey"`
	TweetID     string `json:"tweet_id" gorm:"type:varchar(64);uniqueIndex;not null"`
	TweetURL    string `json:"tweet_url" gorm:"type:varchar(512)"`
	Text        string `json:"text" gorm:"type:text"`
	AnnouncedAt int64  `json:"announced_at" gorm:"bigint;index;not null"` // Unix 秒（UTC）
	CreatedAt   int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt   int64  `json:"updated_at" gorm:"bigint"`
}

func (CodexResetEvent) TableName() string {
	return "codex_reset_events"
}

// CodexResetSyncState 记录最近一次同步结果，供前端展示。
type CodexResetSyncState struct {
	ID             int   `json:"id" gorm:"primaryKey"`
	LastSyncAt     int64 `json:"last_sync_at" gorm:"bigint"`
	LastSuccessAt  int64 `json:"last_success_at" gorm:"bigint"`
	LastErrorAt    int64 `json:"last_error_at" gorm:"bigint"`
	LastError      string `json:"last_error" gorm:"type:text"`
	LastTweetID    string `json:"last_tweet_id" gorm:"type:varchar(64)"`
	LastAnnouncedAt int64 `json:"last_announced_at" gorm:"bigint"`
	TotalEvents    int   `json:"total_events"`
	UpdatedAt      int64 `json:"updated_at" gorm:"bigint"`
}

func (CodexResetSyncState) TableName() string {
	return "codex_reset_sync_state"
}

func (e *CodexResetEvent) BeforeCreate(tx *gorm.DB) error {
	now := time.Now().Unix()
	if e.CreatedAt == 0 {
		e.CreatedAt = now
	}
	e.UpdatedAt = now
	return nil
}

func (e *CodexResetEvent) BeforeUpdate(tx *gorm.DB) error {
	e.UpdatedAt = time.Now().Unix()
	return nil
}

// UpsertCodexResetEvent 按 tweet_id 幂等写入。返回是否新增。
func UpsertCodexResetEvent(event *CodexResetEvent) (created bool, err error) {
	if event == nil || strings.TrimSpace(event.TweetID) == "" {
		return false, errors.New("tweet_id is required")
	}
	var existing CodexResetEvent
	err = DB.Where("tweet_id = ?", event.TweetID).First(&existing).Error
	if err == nil {
		updates := map[string]any{
			"tweet_url":    event.TweetURL,
			"text":         event.Text,
			"announced_at": event.AnnouncedAt,
			"updated_at":   time.Now().Unix(),
		}
		if err := DB.Model(&existing).Updates(updates).Error; err != nil {
			return false, err
		}
		return false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, err
	}
	if err := DB.Create(event).Error; err != nil {
		return false, err
	}
	return true, nil
}

func ListCodexResetEvents(limit int) ([]*CodexResetEvent, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	var events []*CodexResetEvent
	err := DB.Order("announced_at desc").Limit(limit).Find(&events).Error
	return events, err
}

func CountCodexResetEvents() (int64, error) {
	var total int64
	err := DB.Model(&CodexResetEvent{}).Count(&total).Error
	return total, err
}

func GetLatestCodexResetEvent() (*CodexResetEvent, error) {
	var event CodexResetEvent
	err := DB.Order("announced_at desc").First(&event).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &event, nil
}

func GetCodexResetEventByTweetID(tweetID string) (*CodexResetEvent, error) {
	var event CodexResetEvent
	err := DB.Where("tweet_id = ?", tweetID).First(&event).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &event, nil
}

// GetOrCreateCodexResetSyncState 返回单行同步状态（id=1）。
func GetOrCreateCodexResetSyncState() (*CodexResetSyncState, error) {
	var state CodexResetSyncState
	err := DB.First(&state, 1).Error
	if err == nil {
		return &state, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	state = CodexResetSyncState{
		ID:        1,
		UpdatedAt: time.Now().Unix(),
	}
	if err := DB.Create(&state).Error; err != nil {
		// 并发创建时重读
		var existing CodexResetSyncState
		if readErr := DB.First(&existing, 1).Error; readErr == nil {
			return &existing, nil
		}
		return nil, err
	}
	return &state, nil
}

func SaveCodexResetSyncState(state *CodexResetSyncState) error {
	if state == nil {
		return errors.New("sync state is nil")
	}
	if state.ID == 0 {
		state.ID = 1
	}
	state.UpdatedAt = time.Now().Unix()
	return DB.Save(state).Error
}

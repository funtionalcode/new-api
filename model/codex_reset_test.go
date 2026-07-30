package model

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupCodexResetTestDB(t *testing.T) {
	t.Helper()
	originalDB := DB
	t.Cleanup(func() {
		DB = originalDB
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&CodexResetEvent{}, &CodexResetSyncState{}))
	DB = db
}

func TestDeleteCodexResetEventSoftDeletesAndBlocksUpsert(t *testing.T) {
	setupCodexResetTestDB(t)

	created, err := UpsertCodexResetEvent(&CodexResetEvent{
		TweetID:     "tweet-1",
		TweetURL:    "https://example.com/1",
		Text:        "first",
		AnnouncedAt: time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC).Unix(),
	})
	require.NoError(t, err)
	require.True(t, created)

	listed, err := ListCodexResetEvents(10)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	id := listed[0].ID
	require.Positive(t, id)

	deleted, ok, err := DeleteCodexResetEvent(id)
	require.NoError(t, err)
	require.True(t, ok)
	require.NotNil(t, deleted)
	assert.Equal(t, "tweet-1", deleted.TweetID)
	assert.Greater(t, deleted.DeletedAt, int64(0))

	listed, err = ListCodexResetEvents(10)
	require.NoError(t, err)
	assert.Empty(t, listed)

	total, err := CountCodexResetEvents()
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)

	latest, err := GetLatestCodexResetEvent()
	require.NoError(t, err)
	assert.Nil(t, latest)

	// 同步再次写入同 tweet_id 时，应保持软删除且不恢复。
	created, err = UpsertCodexResetEvent(&CodexResetEvent{
		TweetID:     "tweet-1",
		TweetURL:    "https://example.com/1-updated",
		Text:        "should stay deleted",
		AnnouncedAt: time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC).Unix(),
	})
	require.NoError(t, err)
	assert.False(t, created)

	listed, err = ListCodexResetEvents(10)
	require.NoError(t, err)
	assert.Empty(t, listed)

	var stored CodexResetEvent
	require.NoError(t, DB.Where("tweet_id = ?", "tweet-1").First(&stored).Error)
	assert.Greater(t, stored.DeletedAt, int64(0))
	assert.Equal(t, "first", stored.Text)
}

func TestDeleteCodexResetEventMissingID(t *testing.T) {
	setupCodexResetTestDB(t)

	event, ok, err := DeleteCodexResetEvent(999)
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Nil(t, event)
}

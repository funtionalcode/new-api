package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCodexResetsTime(t *testing.T) {
	ts, err := parseCodexResetsTime("2026-07-29T04:09:02.000Z")
	require.NoError(t, err)
	assert.Equal(t, 2026, ts.Year())
	assert.Equal(t, time.July, ts.Month())
	assert.Equal(t, 29, ts.Day())
	assert.Equal(t, 4, ts.Hour())
}

func TestBuildCodexResetsStats(t *testing.T) {
	now := time.Date(2026, 7, 29, 4, 0, 0, 0, time.UTC).Unix()
	events := []*model.CodexResetEvent{
		{TweetID: "3", AnnouncedAt: now},
		{TweetID: "2", AnnouncedAt: now - 2*86400},
		{TweetID: "1", AnnouncedAt: now - 5*86400},
	}
	stats := BuildCodexResetsStats(events)
	assert.Equal(t, 3, stats["total"])
	assert.Equal(t, now, stats["last_reset_at"])
	assert.Equal(t, 2.5, stats["avg_interval_days"])
	assert.Equal(t, 3.0, stats["longest_wait_days"])
	assert.Equal(t, 2.0, stats["shortest_wait_days"])
}

func TestBuildCodexResetsIntervalSeries(t *testing.T) {
	now := time.Date(2026, 7, 29, 0, 0, 0, 0, time.UTC).Unix()
	events := []*model.CodexResetEvent{
		{TweetID: "b", AnnouncedAt: now},
		{TweetID: "a", AnnouncedAt: now - 86400},
	}
	series := BuildCodexResetsIntervalSeries(events)
	require.Len(t, series, 1)
	assert.Equal(t, "a", series[0]["from_tweet_id"])
	assert.Equal(t, "b", series[0]["to_tweet_id"])
	assert.Equal(t, 1.0, series[0]["interval_days"])
}

func TestBuildCodexResetAnnouncementContentTruncates(t *testing.T) {
	event := &model.CodexResetEvent{
		TweetID:     "1",
		AnnouncedAt: time.Date(2026, 7, 29, 4, 9, 2, 0, time.UTC).Unix(),
		Text:        "We have reset usage limits for all Codex users. " + string(make([]byte, 600)),
	}
	content := buildCodexResetAnnouncementContent(event)
	assert.LessOrEqual(t, len([]rune(content)), codexResetContentMaxRunes)
	assert.Contains(t, content, "Codex 用量限制已重置")
}

func TestTruncateRunes(t *testing.T) {
	assert.Equal(t, "你好…", truncateRunes("你好世界", 3))
	assert.Equal(t, "ab", truncateRunes("ab", 5))
}

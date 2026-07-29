package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/console_setting"
)

const (
	codexResetsAPIURL         = "https://codex-resets.com/api/resets"
	codexResetsSourceURL      = "https://codex-resets.com/"
	codexResetsHTTPTimeout    = 20 * time.Second
	codexResetAnnouncementTag = "codex-reset:"
	codexResetContentMaxRunes = 480
)

// CodexResetsRemoteEvent 对应上游 /api/resets 中的 events 元素。
type CodexResetsRemoteEvent struct {
	TweetID     string `json:"tweet_id"`
	TweetURL    string `json:"tweet_url"`
	Text        string `json:"text"`
	AnnouncedAt string `json:"announced_at"`
}

// CodexResetsRemoteStats 对应上游 stats 字段。
type CodexResetsRemoteStats struct {
	Total            int     `json:"total"`
	LastResetAt      string  `json:"last_reset_at"`
	DaysSinceLast    float64 `json:"days_since_last"`
	AvgIntervalDays  float64 `json:"avg_interval_days"`
}

// CodexResetsRemotePayload 对应上游 /api/resets 响应。
type CodexResetsRemotePayload struct {
	Events      []CodexResetsRemoteEvent `json:"events"`
	Stats       CodexResetsRemoteStats   `json:"stats"`
	GeneratedAt string                   `json:"generated_at"`
}

// CodexResetsSyncResult 一次同步的结果摘要。
type CodexResetsSyncResult struct {
	Fetched        int      `json:"fetched"`
	Inserted       int      `json:"inserted"`
	Updated        int      `json:"updated"`
	Announced      int      `json:"announced"`
	TotalEvents    int64    `json:"total_events"`
	LatestTweetID  string   `json:"latest_tweet_id,omitempty"`
	LatestAt       int64    `json:"latest_announced_at,omitempty"`
	NewTweetIDs    []string `json:"new_tweet_ids,omitempty"`
	Source         string   `json:"source"`
	GeneratedAt    string   `json:"generated_at,omitempty"`
	RemoteTotal    int      `json:"remote_total"`
	AvgIntervalDay float64  `json:"avg_interval_days"`
}

// FetchCodexResets 拉取上游重置事件列表。
func FetchCodexResets(ctx context.Context) (*CodexResetsRemotePayload, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, codexResetsAPIURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "new-api-codex-resets-monitor/1.0")

	client := GetHttpClient()
	if client == nil {
		client = &http.Client{Timeout: codexResetsHTTPTimeout}
	}
	// 使用独立超时，避免被全局无限超时 client 拖住
	client = &http.Client{
		Timeout:   codexResetsHTTPTimeout,
		Transport: client.Transport,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer CloseResponseBodyGracefully(resp)

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("codex-resets API returned %d: %s", resp.StatusCode, truncateRunes(string(body), 200))
	}

	var payload CodexResetsRemotePayload
	if err := common.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode codex-resets payload: %w", err)
	}
	return &payload, nil
}

// SyncCodexResets 拉取并落库，对新增事件写入系统公告。
// 首次回填（本地此前无事件）只落库不刷公告，避免历史重置淹没通知中心。
func SyncCodexResets(ctx context.Context, announceNew bool) (*CodexResetsSyncResult, error) {
	state, err := model.GetOrCreateCodexResetSyncState()
	if err != nil {
		return nil, err
	}

	existingCount, err := model.CountCodexResetEvents()
	if err != nil {
		return nil, err
	}
	// 仅当本地已有历史数据时，才对“新增”发公告。
	shouldAnnounce := announceNew && existingCount > 0

	payload, err := FetchCodexResets(ctx)
	now := time.Now().Unix()
	state.LastSyncAt = now
	if err != nil {
		state.LastErrorAt = now
		state.LastError = err.Error()
		_ = model.SaveCodexResetSyncState(state)
		return nil, err
	}

	result := &CodexResetsSyncResult{
		Fetched:        len(payload.Events),
		Source:         codexResetsSourceURL,
		GeneratedAt:    payload.GeneratedAt,
		RemoteTotal:    payload.Stats.Total,
		AvgIntervalDay: payload.Stats.AvgIntervalDays,
		NewTweetIDs:    make([]string, 0),
	}

	// 上游通常按时间倒序；为了公告按时间顺序发布，这里正序处理。
	events := make([]CodexResetsRemoteEvent, len(payload.Events))
	copy(events, payload.Events)
	for i, j := 0, len(events)-1; i < j; i, j = i+1, j-1 {
		events[i], events[j] = events[j], events[i]
	}

	for _, remote := range events {
		tweetID := strings.TrimSpace(remote.TweetID)
		if tweetID == "" {
			continue
		}
		announcedAt, parseErr := parseCodexResetsTime(remote.AnnouncedAt)
		if parseErr != nil {
			common.SysError(fmt.Sprintf("codex-resets skip event %s: %v", tweetID, parseErr))
			continue
		}
		local := &model.CodexResetEvent{
			TweetID:     tweetID,
			TweetURL:    strings.TrimSpace(remote.TweetURL),
			Text:        strings.TrimSpace(remote.Text),
			AnnouncedAt: announcedAt.Unix(),
		}
		created, upsertErr := model.UpsertCodexResetEvent(local)
		if upsertErr != nil {
			return nil, upsertErr
		}
		if created {
			result.Inserted++
			result.NewTweetIDs = append(result.NewTweetIDs, tweetID)
			if shouldAnnounce {
				if publishErr := publishCodexResetAnnouncement(local); publishErr != nil {
					common.SysError(fmt.Sprintf("codex-resets announce %s failed: %v", tweetID, publishErr))
				} else {
					result.Announced++
				}
			}
		} else {
			result.Updated++
		}
	}

	total, countErr := model.CountCodexResetEvents()
	if countErr != nil {
		return nil, countErr
	}
	result.TotalEvents = total

	latest, latestErr := model.GetLatestCodexResetEvent()
	if latestErr != nil {
		return nil, latestErr
	}
	if latest != nil {
		result.LatestTweetID = latest.TweetID
		result.LatestAt = latest.AnnouncedAt
		state.LastTweetID = latest.TweetID
		state.LastAnnouncedAt = latest.AnnouncedAt
	}
	state.TotalEvents = int(total)
	state.LastSuccessAt = now
	state.LastError = ""
	if err := model.SaveCodexResetSyncState(state); err != nil {
		return nil, err
	}
	return result, nil
}

func publishCodexResetAnnouncement(event *model.CodexResetEvent) error {
	if event == nil {
		return nil
	}
	// 确保公告面板开启
	cs := console_setting.GetConsoleSetting()
	if !cs.AnnouncementsEnabled {
		cs.AnnouncementsEnabled = true
		if err := model.UpdateOption("console_setting.announcements_enabled", "true"); err != nil {
			return err
		}
	}

	content := buildCodexResetAnnouncementContent(event)
	extra := "来源: codex-resets.com"
	if event.TweetURL != "" {
		extra = fmt.Sprintf("来源: %s", event.TweetURL)
	}
	publishDate := time.Unix(event.AnnouncedAt, 0).UTC().Format(time.RFC3339)
	if event.AnnouncedAt <= 0 {
		publishDate = time.Now().UTC().Format(time.RFC3339)
	}

	// 用 id 做幂等键，避免重复公告
	announcementID := codexResetAnnouncementTag + event.TweetID

	var list []map[string]any
	if strings.TrimSpace(cs.Announcements) != "" {
		if err := common.UnmarshalJsonStr(cs.Announcements, &list); err != nil {
			// 脏数据时从空列表重建，避免阻塞监控
			common.SysError("codex-resets parse announcements failed, rebuilding list: " + err.Error())
			list = nil
		}
	}
	for _, item := range list {
		if id, ok := item["id"].(string); ok && id == announcementID {
			return nil
		}
	}

	entry := map[string]any{
		"id":          announcementID,
		"content":     content,
		"publishDate": publishDate,
		"type":        "success",
		"extra":       truncateRunes(extra, 200),
	}
	// 新公告放前面
	list = append([]map[string]any{entry}, list...)
	if len(list) > 100 {
		list = list[:100]
	}
	encoded, err := common.Marshal(list)
	if err != nil {
		return err
	}
	if err := console_setting.ValidateConsoleSettings(string(encoded), "Announcements"); err != nil {
		// 内容过长时再截断一次 content
		entry["content"] = truncateRunes(content, 400)
		list[0] = entry
		encoded, err = common.Marshal(list)
		if err != nil {
			return err
		}
		if err := console_setting.ValidateConsoleSettings(string(encoded), "Announcements"); err != nil {
			return err
		}
	}
	return model.UpdateOption("console_setting.announcements", string(encoded))
}

func buildCodexResetAnnouncementContent(event *model.CodexResetEvent) string {
	when := time.Unix(event.AnnouncedAt, 0).UTC().Format("2006-01-02 15:04 UTC")
	snippet := firstLine(event.Text)
	snippet = truncateRunes(snippet, 180)
	content := fmt.Sprintf("Codex 用量限制已重置（%s）。%s", when, snippet)
	return truncateRunes(content, codexResetContentMaxRunes)
}

func parseCodexResetsTime(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, fmt.Errorf("empty announced_at")
	}
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("unsupported time format: %s", raw)
}

func firstLine(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if idx := strings.IndexAny(text, "\r\n"); idx >= 0 {
		return strings.TrimSpace(text[:idx])
	}
	return text
}

func truncateRunes(s string, max int) string {
	if max <= 0 || s == "" {
		return ""
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	if max == 1 {
		return string(runes[:1])
	}
	return string(runes[:max-1]) + "…"
}

// BuildCodexResetsStats 基于本地事件计算统计。
func BuildCodexResetsStats(events []*model.CodexResetEvent) map[string]any {
	stats := map[string]any{
		"total":             0,
		"last_reset_at":     int64(0),
		"days_since_last":   float64(0),
		"avg_interval_days": float64(0),
		"longest_wait_days": float64(0),
		"shortest_wait_days": float64(0),
	}
	if len(events) == 0 {
		return stats
	}

	// events 期望按 announced_at desc
	sorted := make([]*model.CodexResetEvent, 0, len(events))
	for _, e := range events {
		if e != nil && e.AnnouncedAt > 0 {
			sorted = append(sorted, e)
		}
	}
	if len(sorted) == 0 {
		return stats
	}
	// 确保 desc
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].AnnouncedAt > sorted[i].AnnouncedAt {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	latest := sorted[0]
	stats["total"] = len(sorted)
	stats["last_reset_at"] = latest.AnnouncedAt
	daysSince := time.Since(time.Unix(latest.AnnouncedAt, 0).UTC()).Hours() / 24
	if daysSince < 0 {
		daysSince = 0
	}
	stats["days_since_last"] = round1(daysSince)

	if len(sorted) < 2 {
		return stats
	}

	var sum float64
	var longest float64
	shortest := -1.0
	// 从新到旧，间隔 = older 到 newer
	for i := 0; i < len(sorted)-1; i++ {
		newer := sorted[i].AnnouncedAt
		older := sorted[i+1].AnnouncedAt
		if newer <= older {
			continue
		}
		days := float64(newer-older) / 86400
		sum += days
		if days > longest {
			longest = days
		}
		if shortest < 0 || days < shortest {
			shortest = days
		}
	}
	intervals := len(sorted) - 1
	if intervals > 0 && sum > 0 {
		stats["avg_interval_days"] = round1(sum / float64(intervals))
	}
	stats["longest_wait_days"] = round1(longest)
	if shortest >= 0 {
		stats["shortest_wait_days"] = round1(shortest)
	}
	return stats
}

// BuildCodexResetsHeatmap 生成最近 weeks 周的日粒度热力图数据（UTC）。
func BuildCodexResetsHeatmap(events []*model.CodexResetEvent, weeks int) []map[string]any {
	if weeks <= 0 {
		weeks = 26
	}
	if weeks > 52 {
		weeks = 52
	}
	now := time.Now().UTC()
	// 对齐到本周周日（与 codex-resets 网格类似：周日为一周起点）
	weekday := int(now.Weekday()) // Sunday=0
	end := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	start := end.AddDate(0, 0, -(weeks*7 - 1 + weekday))

	counts := make(map[string]int)
	for _, e := range events {
		if e == nil || e.AnnouncedAt <= 0 {
			continue
		}
		day := time.Unix(e.AnnouncedAt, 0).UTC()
		key := day.Format("2006-01-02")
		counts[key]++
	}

	out := make([]map[string]any, 0, weeks*7)
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		count := counts[key]
		level := 0
		if count > 0 {
			level = 1
			if count >= 2 {
				level = 2
			}
			if count >= 3 {
				level = 3
			}
		}
		out = append(out, map[string]any{
			"date":  key,
			"count": count,
			"level": level,
		})
	}
	return out
}

// BuildCodexResetsIntervalSeries 生成相邻重置间隔序列（用于折线/柱状图）。
func BuildCodexResetsIntervalSeries(events []*model.CodexResetEvent) []map[string]any {
	if len(events) < 2 {
		return []map[string]any{}
	}
	// 升序
	sorted := make([]*model.CodexResetEvent, 0, len(events))
	for _, e := range events {
		if e != nil && e.AnnouncedAt > 0 {
			sorted = append(sorted, e)
		}
	}
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].AnnouncedAt < sorted[i].AnnouncedAt {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	series := make([]map[string]any, 0, len(sorted)-1)
	for i := 1; i < len(sorted); i++ {
		prev := sorted[i-1]
		curr := sorted[i]
		if curr.AnnouncedAt <= prev.AnnouncedAt {
			continue
		}
		days := float64(curr.AnnouncedAt-prev.AnnouncedAt) / 86400
		series = append(series, map[string]any{
			"from_tweet_id": prev.TweetID,
			"to_tweet_id":   curr.TweetID,
			"from_at":       prev.AnnouncedAt,
			"to_at":         curr.AnnouncedAt,
			"date":          time.Unix(curr.AnnouncedAt, 0).UTC().Format("2006-01-02"),
			"interval_days": round1(days),
		})
	}
	return series
}

func round1(v float64) float64 {
	return float64(int(v*10+0.5)) / 10
}

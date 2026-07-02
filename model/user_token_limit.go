package model

import (
	"fmt"
	"time"
)

type TokenLimitPeriod string

const (
	TokenLimitPeriodDaily   TokenLimitPeriod = "daily"
	TokenLimitPeriodWeekly  TokenLimitPeriod = "weekly"
	TokenLimitPeriodMonthly TokenLimitPeriod = "monthly"
)

type UserTokenPeriodUsage struct {
	Daily   int64 `json:"daily"`
	Weekly  int64 `json:"weekly"`
	Monthly int64 `json:"monthly"`
}

type UserTokenLimitResult struct {
	Exceeded bool                 `json:"exceeded"`
	Period   TokenLimitPeriod     `json:"period,omitempty"`
	Used     int64                `json:"used,omitempty"`
	Limit    int64                `json:"limit,omitempty"`
	Usage    UserTokenPeriodUsage `json:"usage"`
	Limits   UserTokenPeriodUsage `json:"limits"`
}

func (r UserTokenLimitResult) Message() string {
	if !r.Exceeded {
		return ""
	}
	periodName := map[TokenLimitPeriod]string{
		TokenLimitPeriodDaily:   "每日",
		TokenLimitPeriodWeekly:  "每周",
		TokenLimitPeriodMonthly: "每月",
	}[r.Period]
	if periodName == "" {
		periodName = "周期"
	}
	return fmt.Sprintf("用户%s Token 使用量已达上限，已用 %d / 上限 %d", periodName, r.Used, r.Limit)
}

func GetUserTokenLimits(userId int) (UserTokenPeriodUsage, error) {
	var user User
	err := DB.Model(&User{}).
		Select("daily_token_limit", "weekly_token_limit", "monthly_token_limit").
		Where("id = ?", userId).
		First(&user).Error
	if err != nil {
		return UserTokenPeriodUsage{}, err
	}
	return UserTokenPeriodUsage{
		Daily:   user.DailyTokenLimit,
		Weekly:  user.WeeklyTokenLimit,
		Monthly: user.MonthlyTokenLimit,
	}, nil
}

func GetUserTokenPeriodUsage(userId int, now time.Time) (UserTokenPeriodUsage, error) {
	if now.IsZero() {
		now = time.Now()
	}
	dayStart := startOfDay(now)
	weekStart := startOfWeek(now)
	monthStart := startOfMonth(now)
	lowerBound := minTime(dayStart, weekStart, monthStart).Unix()

	var usage UserTokenPeriodUsage
	err := LOG_DB.Table("logs").
		Select(`
			coalesce(sum(case when created_at >= ? then prompt_tokens + completion_tokens else 0 end), 0) as daily,
			coalesce(sum(case when created_at >= ? then prompt_tokens + completion_tokens else 0 end), 0) as weekly,
			coalesce(sum(case when created_at >= ? then prompt_tokens + completion_tokens else 0 end), 0) as monthly
		`, dayStart.Unix(), weekStart.Unix(), monthStart.Unix()).
		Where("user_id = ? and type = ? and created_at >= ? and created_at <= ?", userId, LogTypeConsume, lowerBound, now.Unix()).
		Scan(&usage).Error
	if err != nil {
		return UserTokenPeriodUsage{}, err
	}
	return usage, nil
}

func CheckUserTokenLimit(userId int, now time.Time) (UserTokenLimitResult, error) {
	limits, err := GetUserTokenLimits(userId)
	if err != nil {
		return UserTokenLimitResult{}, err
	}

	result := UserTokenLimitResult{Limits: limits}
	if limits.Daily <= 0 && limits.Weekly <= 0 && limits.Monthly <= 0 {
		return result, nil
	}

	usage, err := GetUserTokenPeriodUsage(userId, now)
	if err != nil {
		return UserTokenLimitResult{}, err
	}
	result.Usage = usage

	if limits.Daily > 0 && usage.Daily >= limits.Daily {
		result.Exceeded = true
		result.Period = TokenLimitPeriodDaily
		result.Used = usage.Daily
		result.Limit = limits.Daily
		return result, nil
	}
	if limits.Weekly > 0 && usage.Weekly >= limits.Weekly {
		result.Exceeded = true
		result.Period = TokenLimitPeriodWeekly
		result.Used = usage.Weekly
		result.Limit = limits.Weekly
		return result, nil
	}
	if limits.Monthly > 0 && usage.Monthly >= limits.Monthly {
		result.Exceeded = true
		result.Period = TokenLimitPeriodMonthly
		result.Used = usage.Monthly
		result.Limit = limits.Monthly
		return result, nil
	}

	return result, nil
}

func startOfDay(now time.Time) time.Time {
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}

func startOfWeek(now time.Time) time.Time {
	dayStart := startOfDay(now)
	daysSinceMonday := (int(dayStart.Weekday()) + 6) % 7
	return dayStart.AddDate(0, 0, -daysSinceMonday)
}

func startOfMonth(now time.Time) time.Time {
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
}

func minTime(times ...time.Time) time.Time {
	min := times[0]
	for _, t := range times[1:] {
		if t.Before(min) {
			min = t
		}
	}
	return min
}

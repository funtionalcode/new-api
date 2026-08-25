package controller

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

const (
	cliproxyWhamUsageURL         = "https://chatgpt.com/backend-api/wham/usage"
	cliproxyClaudeUsageURL       = "https://api.anthropic.com/api/oauth/usage"
	cliproxyClaudeProfileURL     = "https://api.anthropic.com/api/oauth/profile"
	cliproxyXAIWeeklyBillingURL  = "https://cli-chat-proxy.grok.com/v1/billing?format=credits"
	cliproxyXAIMonthlyBillingURL = "https://cli-chat-proxy.grok.com/v1/billing"
	cliproxyXAISettingsURL       = "https://cli-chat-proxy.grok.com/v1/settings"
	cliproxyXAIBillingURL        = cliproxyXAIMonthlyBillingURL
)

type cliproxyXAIProductUsage struct {
	Product      string `json:"product"`
	UsagePercent int    `json:"usage_percent"`
}

type cliproxyAPICaller interface {
	CallAPI(context.Context, service.CliproxyAPICallRequest) (*service.CliproxyAPICallResponse, error)
}

type cliproxyXAICallResult struct {
	Response *service.CliproxyAPICallResponse
	Err      error
}

type cliproxyXAIRefreshResult struct {
	Usage             cliproxyUsageRefreshBody
	Warning           string
	AllowPartialUsage bool
}

type cliproxyAuthFileBindingRequest struct {
	UserId       int    `json:"user_id"`
	AuthIndex    string `json:"auth_index"`
	AuthName     string `json:"auth_name"`
	AuthFile     string `json:"auth_file"`
	Note         string `json:"note"`
	AccountId    string `json:"account_id"`
	LastPlanType string `json:"last_plan_type"`
	Enabled      bool   `json:"enabled"`
}

type cliproxyUsageRefreshBody struct {
	UsedTokens               int
	Quota                    int
	PlanType                 string
	FiveHourPercent          int
	FiveHourResetAt          int64
	WeeklyPercent            int
	WeeklyResetAt            int64
	CodexFiveHourPercent     int
	CodexFiveHourResetAt     int64
	CodexWeeklyPercent       int
	CodexWeeklyResetAt       int64
	ClaudeFablePercent       int
	ClaudeFableResetAt       int64
	OnDemandCap              int
	BillingPeriodEndAt       int64
	XAIWeeklyPercent         int
	XAIWeeklyPercentSet      bool
	XAIWeeklyPeriodStartAt   int64
	XAIWeeklyPeriodEndAt     int64
	XAIProductUsage          string
	XAIOnDemandUsed          int
	XAIOnDemandUsedRefreshed bool
}

func GetCliproxyRemoteAuthFiles(c *gin.Context) {
	client, err := newCliproxyClientFromOptions()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	files, err := client.ListAuthFiles(c.Request.Context())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, files)
}

func GetCliproxyAuthFileBindings(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	query := cliproxyBindingQueryForRole(model.CliproxyAuthFileBindingQuery{
		Username:  c.Query("username"),
		AuthIndex: c.Query("auth_index"),
		Enabled:   parseOptionalBool(c.Query("enabled")),
	}, c.GetInt("id"), c.GetInt("role"))
	bindings, total, err := model.GetCliproxyAuthFileBindings(query, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(bindings)
	common.ApiSuccess(c, pageInfo)
}

func cliproxyBindingQueryForRole(query model.CliproxyAuthFileBindingQuery, userID int, role int) model.CliproxyAuthFileBindingQuery {
	if role >= common.RoleAdminUser {
		return query
	}
	query.UserId = userID
	query.Username = ""
	return query
}

func canAccessCliproxyBindingForRole(binding *model.CliproxyAuthFileBinding, userID int, role int) bool {
	return binding != nil && (role >= common.RoleAdminUser || binding.UserId == userID)
}

func CreateCliproxyAuthFileBinding(c *gin.Context) {
	update, err := decodeCliproxyAuthFileBindingRequest(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	binding := &model.CliproxyAuthFileBinding{
		UserId:       update.UserId,
		Username:     update.Username,
		AuthIndex:    update.AuthIndex,
		AuthName:     update.AuthName,
		AuthFile:     update.AuthFile,
		Note:         update.Note,
		AccountId:    update.AccountId,
		LastPlanType: update.LastPlanType,
		Enabled:      update.Enabled,
	}
	if err := model.CreateCliproxyAuthFileBinding(binding); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, binding)
}

func UpdateCliproxyAuthFileBinding(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("认证文件绑定 ID 无效"))
		return
	}
	update, err := decodeCliproxyAuthFileBindingRequest(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	binding, err := model.UpdateCliproxyAuthFileBinding(id, update)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, binding)
}

func DeleteCliproxyAuthFileBinding(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("认证文件绑定 ID 无效"))
		return
	}
	if err := model.DeleteCliproxyAuthFileBindingById(id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func RefreshCliproxyAuthFileBindingUsage(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("认证文件绑定 ID 无效"))
		return
	}
	binding, err := model.GetCliproxyAuthFileBindingById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !canAccessCliproxyBindingForRole(binding, c.GetInt("id"), c.GetInt("role")) {
		common.ApiError(c, fmt.Errorf("无权刷新该认证文件绑定"))
		return
	}
	client, err := newCliproxyClientFromOptions()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if isCliproxyXAIAuthFile(binding) {
		refresh, refreshErr := refreshCliproxyXAIUsage(c.Request.Context(), client, binding)
		if refreshErr != nil {
			_, updateErr := model.UpdateCliproxyAuthFileBindingUsage(id, model.CliproxyUsageRefreshUpdate{
				LastError:               refreshErr.Error(),
				PreserveLastRefreshedAt: true,
			})
			if updateErr != nil {
				common.ApiError(c, fmt.Errorf("刷新额度失败: %s；保存错误失败: %w", refreshErr.Error(), updateErr))
				return
			}
			common.ApiError(c, refreshErr)
			return
		}
		updatedBinding, updateErr := model.UpdateCliproxyAuthFileBindingUsage(id, model.CliproxyUsageRefreshUpdate{
			LastUsageTokens:              refresh.Usage.UsedTokens,
			LastUsageQuota:               refresh.Usage.Quota,
			LastPlanType:                 refresh.Usage.PlanType,
			LastXAIWeeklyPercent:         refresh.Usage.XAIWeeklyPercent,
			LastXAIWeeklyPeriodStartAt:   refresh.Usage.XAIWeeklyPeriodStartAt,
			LastXAIWeeklyPeriodEndAt:     refresh.Usage.XAIWeeklyPeriodEndAt,
			LastXAIProductUsage:          refresh.Usage.XAIProductUsage,
			LastXAIOnDemandCap:           refresh.Usage.OnDemandCap,
			LastXAIOnDemandUsed:          refresh.Usage.XAIOnDemandUsed,
			LastXAIOnDemandUsedRefreshed: refresh.Usage.XAIOnDemandUsedRefreshed,
			LastXAIBillingPeriodEndAt:    refresh.Usage.BillingPeriodEndAt,
			LastError:                    refresh.Warning,
			AllowPartialUsage:            refresh.AllowPartialUsage,
		})
		if updateErr != nil {
			common.ApiError(c, updateErr)
			return
		}
		common.ApiSuccess(c, updatedBinding)
		return
	}
	result, err := client.CallAPI(c.Request.Context(), buildCliproxyUsageRefreshRequest(binding))
	if err != nil {
		updatedBinding, updateErr := model.UpdateCliproxyAuthFileBindingUsage(id, model.CliproxyUsageRefreshUpdate{LastError: err.Error()})
		if updateErr != nil {
			common.ApiError(c, fmt.Errorf("刷新额度失败: %s；保存错误失败: %w", err.Error(), updateErr))
			return
		}
		common.ApiSuccess(c, updatedBinding)
		return
	}
	usage, usageErr := extractCliproxyUsage(result)
	if usageErr != nil {
		updatedBinding, updateErr := model.UpdateCliproxyAuthFileBindingUsage(id, model.CliproxyUsageRefreshUpdate{LastError: usageErr.Error()})
		if updateErr != nil {
			common.ApiError(c, fmt.Errorf("解析额度失败: %s；保存错误失败: %w", usageErr.Error(), updateErr))
			return
		}
		common.ApiSuccess(c, updatedBinding)
		return
	}
	if isCliproxyClaudeAuthFile(binding) {
		usage.PlanType = firstNonEmpty(fetchCliproxyClaudeProfilePlan(c.Request.Context(), client, binding.AuthIndex), binding.LastPlanType, usage.PlanType)
	}
	updatedBinding, err := model.UpdateCliproxyAuthFileBindingUsage(id, model.CliproxyUsageRefreshUpdate{
		LastUsageTokens:           usage.UsedTokens,
		LastUsageQuota:            usage.Quota,
		LastPlanType:              usage.PlanType,
		LastFiveHourPercent:       usage.FiveHourPercent,
		LastFiveHourResetAt:       usage.FiveHourResetAt,
		LastWeeklyPercent:         usage.WeeklyPercent,
		LastWeeklyResetAt:         usage.WeeklyResetAt,
		LastCodexFiveHourPercent:  usage.CodexFiveHourPercent,
		LastCodexFiveHourResetAt:  usage.CodexFiveHourResetAt,
		LastCodexWeeklyPercent:    usage.CodexWeeklyPercent,
		LastCodexWeeklyResetAt:    usage.CodexWeeklyResetAt,
		LastClaudeFablePercent:    usage.ClaudeFablePercent,
		LastClaudeFableResetAt:    usage.ClaudeFableResetAt,
		LastXAIOnDemandCap:        usage.OnDemandCap,
		LastXAIBillingPeriodEndAt: usage.BillingPeriodEndAt,
		LastError:                 "",
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, updatedBinding)
}

func GetCliproxyUserConsumption(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	channelId, _ := strconv.Atoi(c.Query("channel_id"))
	query := model.UserTokenUsageQuery{
		StartTimestamp: startTimestamp,
		EndTimestamp:   endTimestamp,
		Username:       c.Query("username"),
		TokenName:      c.Query("token_name"),
		AuthIndex:      c.Query("auth_index"),
		ChannelId:      channelId,
		ModelName:      c.Query("model_name"),
		SortBy:         c.Query("sort_by"),
		SortOrder:      c.Query("sort_order"),
	}
	if c.GetInt("role") < common.RoleAdminUser {
		query.UserId = c.GetInt("id")
		query.Username = ""
	}
	if c.Query("group_by") == "day" {
		summaries, total, err := model.GetUserTokenUsageByDay(query, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
		if err != nil {
			common.ApiError(c, err)
			return
		}
		pageInfo.SetTotal(int(total))
		pageInfo.SetItems(summaries)
		common.ApiSuccess(c, pageInfo)
		return
	}
	summaries, total, err := model.GetUserTokenUsageSummary(query, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(summaries)
	common.ApiSuccess(c, pageInfo)
}

func decodeCliproxyAuthFileBindingRequest(c *gin.Context) (model.CliproxyAuthFileBindingUpdate, error) {
	var request cliproxyAuthFileBindingRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		return model.CliproxyAuthFileBindingUpdate{}, fmt.Errorf("无效的参数")
	}
	user, err := model.GetUserById(request.UserId, false)
	if err != nil {
		return model.CliproxyAuthFileBindingUpdate{}, err
	}
	update := model.CliproxyAuthFileBindingUpdate{
		UserId:       user.Id,
		Username:     user.Username,
		AuthIndex:    strings.TrimSpace(request.AuthIndex),
		AuthName:     strings.TrimSpace(request.AuthName),
		AuthFile:     request.AuthFile,
		Note:         request.Note,
		AccountId:    strings.TrimSpace(request.AccountId),
		LastPlanType: strings.TrimSpace(request.LastPlanType),
		Enabled:      request.Enabled,
	}
	if err := model.ValidateCliproxyAuthFileBindingUpdate(update); err != nil {
		return model.CliproxyAuthFileBindingUpdate{}, err
	}
	return update, nil
}

func newCliproxyClientFromOptions() (*service.CliproxyAPIClient, error) {
	common.OptionMapRWMutex.RLock()
	baseURL := common.OptionMap["CliproxyAPIBaseURL"]
	password := common.OptionMap["CliproxyAPIPassword"]
	common.OptionMapRWMutex.RUnlock()
	return service.NewCliproxyAPIClient(baseURL, password)
}

func buildCliproxyUsageRefreshRequest(binding *model.CliproxyAuthFileBinding) service.CliproxyAPICallRequest {
	if isCliproxyXAIAuthFile(binding) {
		return buildCliproxyXAIBillingRequest(binding.AuthIndex)
	}
	if isCliproxyClaudeAuthFile(binding) {
		return buildCliproxyClaudeUsageRequest(binding.AuthIndex)
	}
	return service.CliproxyAPICallRequest{
		AuthIndex: binding.AuthIndex,
		Method:    http.MethodGet,
		URL:       cliproxyWhamUsageURL,
		Header: map[string]string{
			"Authorization":      "Bearer $TOKEN$",
			"Content-Type":       "application/json",
			"User-Agent":         "codex_cli_rs/0.76.0 (Debian 13.0.0; x86_64) WindowsTerminal",
			"Chatgpt-Account-Id": binding.AccountId,
		},
	}
}

func buildCliproxyXAIBillingRequest(authIndex string) service.CliproxyAPICallRequest {
	return buildCliproxyXAIMonthlyBillingRequest(authIndex)
}

func buildCliproxyXAIWeeklyBillingRequest(authIndex string) service.CliproxyAPICallRequest {
	return buildCliproxyXAIRequestForURL(authIndex, cliproxyXAIWeeklyBillingURL)
}

func buildCliproxyXAIMonthlyBillingRequest(authIndex string) service.CliproxyAPICallRequest {
	return buildCliproxyXAIRequestForURL(authIndex, cliproxyXAIMonthlyBillingURL)
}

func buildCliproxyXAISettingsRequest(authIndex string) service.CliproxyAPICallRequest {
	return buildCliproxyXAIRequestForURL(authIndex, cliproxyXAISettingsURL)
}

func buildCliproxyXAIRequestForURL(authIndex string, url string) service.CliproxyAPICallRequest {
	return service.CliproxyAPICallRequest{
		AuthIndex: authIndex,
		Method:    http.MethodGet,
		URL:       url,
		Header: map[string]string{
			"Authorization":         "Bearer $TOKEN$",
			"x-xai-token-auth":      "xai-grok-cli",
			"x-grok-client-version": "0.2.91",
			"accept":                "*/*",
			"user-agent":            "grok-pager/0.2.91 grok-shell/0.2.91 (macos; aarch64)",
		},
	}
}

func refreshCliproxyXAIUsage(ctx context.Context, caller cliproxyAPICaller, binding *model.CliproxyAuthFileBinding) (cliproxyXAIRefreshResult, error) {
	// Fetch weekly and monthly snapshots sequentially. Concurrent calls for the
	// same xAI auth index are more likely to hit upstream rate limits during bulk refresh.
	weeklyResponse, weeklyCallErr := caller.CallAPI(ctx, buildCliproxyXAIWeeklyBillingRequest(binding.AuthIndex))
	weeklyResult := cliproxyXAICallResult{Response: weeklyResponse, Err: weeklyCallErr}
	monthlyResponse, monthlyCallErr := caller.CallAPI(ctx, buildCliproxyXAIMonthlyBillingRequest(binding.AuthIndex))
	monthlyResult := cliproxyXAICallResult{Response: monthlyResponse, Err: monthlyCallErr}
	weeklyUsage, weeklyErr := extractCliproxyXAIUsageResult(weeklyResult, resolveCliproxyXAIWeeklyUsage)
	monthlyUsage, monthlyErr := extractCliproxyXAIUsageResult(monthlyResult, resolveCliproxyXAIMonthlyUsage)
	if weeklyErr != nil && monthlyErr != nil {
		return cliproxyXAIRefreshResult{}, fmt.Errorf("周额度刷新失败: %v；月度额度刷新失败: %v", weeklyErr, monthlyErr)
	}
	settingsResponse, settingsErr := caller.CallAPI(ctx, buildCliproxyXAISettingsRequest(binding.AuthIndex))
	settingsPlan := ""
	if settingsErr == nil {
		settingsPlan = extractCliproxyXAIPlan(settingsResponse)
	}

	usage := cliproxyUsageRefreshBody{
		UsedTokens:               binding.LastUsageTokens,
		Quota:                    binding.LastUsageQuota,
		PlanType:                 binding.LastPlanType,
		OnDemandCap:              binding.LastXAIOnDemandCap,
		BillingPeriodEndAt:       binding.LastXAIBillingPeriodEndAt,
		XAIWeeklyPercent:         binding.LastXAIWeeklyPercent,
		XAIWeeklyPeriodStartAt:   binding.LastXAIWeeklyPeriodStartAt,
		XAIWeeklyPeriodEndAt:     binding.LastXAIWeeklyPeriodEndAt,
		XAIProductUsage:          binding.LastXAIProductUsage,
		XAIOnDemandUsed:          binding.LastXAIOnDemandUsed,
		XAIOnDemandUsedRefreshed: binding.LastXAIOnDemandUsedRefreshed,
	}
	if weeklyErr == nil {
		usage.XAIWeeklyPercent = weeklyUsage.XAIWeeklyPercent
		usage.XAIWeeklyPeriodStartAt = weeklyUsage.XAIWeeklyPeriodStartAt
		usage.XAIWeeklyPeriodEndAt = weeklyUsage.XAIWeeklyPeriodEndAt
		usage.XAIProductUsage = weeklyUsage.XAIProductUsage
	}
	if monthlyErr == nil {
		usage.UsedTokens = monthlyUsage.UsedTokens
		usage.Quota = monthlyUsage.Quota
		// A successful monthly refresh without an explicit subscription tier must
		// clear values previously guessed from quota amounts.
		usage.PlanType = monthlyUsage.PlanType
		usage.OnDemandCap = monthlyUsage.OnDemandCap
		usage.XAIOnDemandUsed = monthlyUsage.XAIOnDemandUsed
		usage.XAIOnDemandUsedRefreshed = monthlyUsage.XAIOnDemandUsedRefreshed
		usage.BillingPeriodEndAt = monthlyUsage.BillingPeriodEndAt
	}
	if settingsPlan != "" {
		usage.PlanType = settingsPlan
	}
	if weeklyErr == nil && monthlyErr == nil && !weeklyUsage.XAIWeeklyPercentSet && usage.Quota > 0 {
		weeklyPercent := math.Round(float64(usage.UsedTokens) / float64(usage.Quota) * 100)
		usage.XAIWeeklyPercent = int(math.Max(0, math.Min(100, weeklyPercent)))
	}

	refresh := cliproxyXAIRefreshResult{Usage: usage}
	if weeklyErr != nil {
		refresh.Warning = fmt.Sprintf("周额度刷新失败: %v", weeklyErr)
		refresh.AllowPartialUsage = true
	}
	if monthlyErr != nil {
		refresh.Warning = fmt.Sprintf("月度额度刷新失败: %v", monthlyErr)
		refresh.AllowPartialUsage = true
	}
	return refresh, nil
}

func extractCliproxyXAIUsageResult(result cliproxyXAICallResult, resolver func(map[string]any) (cliproxyUsageRefreshBody, bool)) (cliproxyUsageRefreshBody, error) {
	if result.Err != nil {
		return cliproxyUsageRefreshBody{}, result.Err
	}
	if result.Response == nil {
		return cliproxyUsageRefreshBody{}, fmt.Errorf("刷新结果为空")
	}
	body := result.Response.Body
	if len(body) == 0 {
		body = result.Response.Data
	}
	usage, ok := resolver(body)
	if !ok {
		return cliproxyUsageRefreshBody{}, fmt.Errorf("刷新结果格式无效")
	}
	return usage, nil
}

func buildCliproxyClaudeUsageRequest(authIndex string) service.CliproxyAPICallRequest {
	return buildCliproxyClaudeOAuthRequest(authIndex, cliproxyClaudeUsageURL)
}

func buildCliproxyClaudeProfileRequest(authIndex string) service.CliproxyAPICallRequest {
	return buildCliproxyClaudeOAuthRequest(authIndex, cliproxyClaudeProfileURL)
}

func buildCliproxyClaudeOAuthRequest(authIndex string, url string) service.CliproxyAPICallRequest {
	return service.CliproxyAPICallRequest{
		AuthIndex: authIndex,
		Method:    http.MethodGet,
		URL:       url,
		Header: map[string]string{
			"Authorization":  "Bearer $TOKEN$",
			"Content-Type":   "application/json",
			"anthropic-beta": "oauth-2025-04-20",
		},
	}
}

func fetchCliproxyClaudeProfilePlan(ctx context.Context, client *service.CliproxyAPIClient, authIndex string) string {
	result, err := client.CallAPI(ctx, buildCliproxyClaudeProfileRequest(authIndex))
	if err != nil || result == nil {
		return ""
	}
	body := result.Body
	if len(body) == 0 {
		body = result.Data
	}
	return resolveCliproxyClaudeProfilePlan(body)
}

func extractCliproxyUsage(result *service.CliproxyAPICallResponse) (cliproxyUsageRefreshBody, error) {
	if result == nil {
		return cliproxyUsageRefreshBody{}, fmt.Errorf("刷新结果为空")
	}
	body := result.Body
	if len(body) == 0 {
		body = result.Data
	}
	if len(body) == 0 {
		return cliproxyUsageRefreshBody{}, fmt.Errorf("刷新结果缺少用量数据")
	}
	if xaiUsage, ok := resolveCliproxyXAIUsage(body); ok {
		return xaiUsage, nil
	}
	fiveHourWindow, weeklyWindow := resolveCliproxyUsageWindows(body)
	claudeFiveHourWindow, claudeWeeklyWindow, claudeFableWindow := resolveCliproxyClaudeUsageWindows(body)
	if len(fiveHourWindow) == 0 && len(weeklyWindow) == 0 && len(claudeFiveHourWindow) == 0 && len(claudeWeeklyWindow) == 0 && len(claudeFableWindow) == 0 && stringFromMap(body, "plan_type") == "" {
		return cliproxyUsageRefreshBody{}, fmt.Errorf("刷新结果格式无效")
	}
	if len(claudeFiveHourWindow) > 0 || len(claudeWeeklyWindow) > 0 || len(claudeFableWindow) > 0 {
		return cliproxyUsageRefreshBody{
			UsedTokens:         intFromMap(body, "used_tokens"),
			Quota:              intFromMap(body, "quota"),
			PlanType:           firstNonEmpty(stringFromMap(body, "plan_type"), "claude"),
			FiveHourPercent:    percentFromWindow(claudeFiveHourWindow),
			FiveHourResetAt:    resetAtFromWindow(claudeFiveHourWindow),
			WeeklyPercent:      percentFromWindow(claudeWeeklyWindow),
			WeeklyResetAt:      resetAtFromWindow(claudeWeeklyWindow),
			ClaudeFablePercent: percentFromWindow(claudeFableWindow),
			ClaudeFableResetAt: resetAtFromWindow(claudeFableWindow),
		}, nil
	}
	codexFiveHourWindow, codexWeeklyWindow := resolveCliproxyCodexUsageWindows(body)
	return cliproxyUsageRefreshBody{
		UsedTokens:           intFromMap(body, "used_tokens"),
		Quota:                intFromMap(body, "quota"),
		PlanType:             stringFromMap(body, "plan_type"),
		FiveHourPercent:      percentFromWindow(fiveHourWindow),
		FiveHourResetAt:      resetAtFromWindow(fiveHourWindow),
		WeeklyPercent:        percentFromWindow(weeklyWindow),
		WeeklyResetAt:        resetAtFromWindow(weeklyWindow),
		CodexFiveHourPercent: percentFromWindow(codexFiveHourWindow),
		CodexFiveHourResetAt: resetAtFromWindow(codexFiveHourWindow),
		CodexWeeklyPercent:   percentFromWindow(codexWeeklyWindow),
		CodexWeeklyResetAt:   resetAtFromWindow(codexWeeklyWindow),
	}, nil
}

func resolveCliproxyUsageWindows(body map[string]any) (map[string]any, map[string]any) {
	rateLimit := mapFromMap(body, "rate_limit")
	primaryWindow := mapFromMap(rateLimit, "primary_window")
	secondaryWindow := mapFromMap(rateLimit, "secondary_window")
	return classifyCliproxyUsageWindows(primaryWindow, secondaryWindow)
}

func resolveCliproxyCodexUsageWindows(body map[string]any) (map[string]any, map[string]any) {
	for _, item := range sliceFromMap(body, "additional_rate_limits") {
		itemMap, ok := item.(map[string]any)
		if !ok || !strings.Contains(strings.ToLower(stringFromMap(itemMap, "limit_name")+" "+stringFromMap(itemMap, "metered_feature")), "codex") {
			continue
		}
		rateLimit := mapFromMap(itemMap, "rate_limit")
		primaryWindow := mapFromMap(rateLimit, "primary_window")
		secondaryWindow := mapFromMap(rateLimit, "secondary_window")
		return classifyCliproxyUsageWindows(primaryWindow, secondaryWindow)
	}
	return nil, nil
}

func resolveCliproxyClaudeUsageWindows(body map[string]any) (map[string]any, map[string]any, map[string]any) {
	fiveHourWindow := mapFromMap(body, "five_hour")
	weeklyWindow := mapFromMap(body, "seven_day")
	var fableWindow map[string]any

	for _, item := range sliceFromMap(body, "limits") {
		limit, ok := item.(map[string]any)
		if !ok || !hasCliproxyUsageWindowData(limit) {
			continue
		}

		kind := normalizeCliproxyPlan(stringFromMap(limit, "kind"))
		group := normalizeCliproxyPlan(stringFromMap(limit, "group"))
		scope := mapFromMap(limit, "scope")
		if len(scope) == 0 {
			if active, activeSet := boolFromMap(limit, "is_active"); activeSet && !active {
				continue
			}
			if active, activeSet := boolFromMap(limit, "isActive"); activeSet && !active {
				continue
			}
			if len(fiveHourWindow) == 0 && kind == "session" && (group == "" || group == "session") {
				fiveHourWindow = limit
			}
			if len(weeklyWindow) == 0 && (kind == "weekly" || kind == "weeklyall") && (group == "" || group == "weekly" || group == "weeklyall") {
				weeklyWindow = limit
			}
			continue
		}

		isWeeklyScoped := kind == "weeklyscoped" || kind == "weeklymodelscoped" || (kind == "modelscoped" && group == "weekly")
		if !isWeeklyScoped || (group != "" && group != "weekly") {
			continue
		}
		modelData := mapFromMap(scope, "model")
		displayName := firstStringFromMap(modelData, "display_name", "displayName", "name")
		if displayName == "" {
			displayName = firstStringFromMap(mapFromMap(modelData, "details"), "display_name", "displayName", "name")
		}
		if !strings.Contains(strings.ToLower(displayName), "fable") {
			continue
		}

		if len(fableWindow) == 0 {
			fableWindow = limit
			continue
		}
		candidateResetAt := resetAtFromWindow(limit)
		currentResetAt := resetAtFromWindow(fableWindow)
		candidateActive, candidateActiveSet := boolFromMap(limit, "is_active")
		if !candidateActiveSet {
			candidateActive, candidateActiveSet = boolFromMap(limit, "isActive")
		}
		currentActive, currentActiveSet := boolFromMap(fableWindow, "is_active")
		if !currentActiveSet {
			currentActive, currentActiveSet = boolFromMap(fableWindow, "isActive")
		}
		if candidateResetAt > currentResetAt || (candidateResetAt == currentResetAt && candidateActiveSet && candidateActive && (!currentActiveSet || !currentActive)) {
			fableWindow = limit
		}
	}

	return fiveHourWindow, weeklyWindow, fableWindow
}

func resolveCliproxyXAIUsage(body map[string]any) (cliproxyUsageRefreshBody, bool) {
	return resolveCliproxyXAIMonthlyUsage(body)
}

func resolveCliproxyXAIWeeklyUsage(body map[string]any) (cliproxyUsageRefreshBody, bool) {
	config := cliproxyXAIPayload(body)
	currentPeriod := firstMapFromMap(config, "currentPeriod", "current_period")
	_, hasCamelCaseUsagePercent := config["creditUsagePercent"]
	_, hasSnakeCaseUsagePercent := config["credit_usage_percent"]
	usagePercent := firstIntFromMap(config, "creditUsagePercent", "credit_usage_percent")
	productItems := firstSliceFromMap(config, "productUsage", "product_usage")
	if len(currentPeriod) == 0 && usagePercent == 0 && len(productItems) == 0 {
		return cliproxyUsageRefreshBody{}, false
	}

	productUsage := make([]cliproxyXAIProductUsage, 0, len(productItems))
	for _, item := range productItems {
		itemMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		product := firstNonEmpty(stringFromMap(itemMap, "product"), fmt.Sprintf("Product %d", len(productUsage)+1))
		productUsage = append(productUsage, cliproxyXAIProductUsage{
			Product:      product,
			UsagePercent: firstIntFromMap(itemMap, "usagePercent", "usage_percent"),
		})
	}
	productUsageJSON, err := common.Marshal(productUsage)
	if err != nil {
		return cliproxyUsageRefreshBody{}, false
	}

	return cliproxyUsageRefreshBody{
		XAIWeeklyPercent:       usagePercent,
		XAIWeeklyPercentSet:    hasCamelCaseUsagePercent || hasSnakeCaseUsagePercent,
		XAIWeeklyPeriodStartAt: unixFromRFC3339(firstStringFromMap(currentPeriod, "start")),
		XAIWeeklyPeriodEndAt:   unixFromRFC3339(firstStringFromMap(currentPeriod, "end")),
		XAIProductUsage:        string(productUsageJSON),
	}, true
}

func resolveCliproxyXAIMonthlyUsage(body map[string]any) (cliproxyUsageRefreshBody, bool) {
	config := cliproxyXAIPayload(body)
	planType := resolveCliproxyXAIPlan(body)
	usageData := firstMapFromMap(config, "usage")
	billingCycle := firstMapFromMap(config, "billingCycle", "billing_cycle")
	quota, hasQuota := cliproxyXAICents(
		config,
		"monthlyLimit",
		"monthly_limit",
		"monthlyBillableLimit",
		"monthly_billable_limit",
	)
	usedTokens, hasUsed := cliproxyXAICents(config, "used")
	if !hasUsed {
		usedTokens, hasUsed = cliproxyXAICents(
			usageData,
			"includedUsed",
			"included_used",
			"totalUsed",
			"total_used",
			"used",
		)
	}
	onDemandCap, hasOnDemandCap := cliproxyXAICents(config, "onDemandCap", "on_demand_cap")
	onDemandUsed, hasOnDemandUsed := cliproxyXAICents(config, "onDemandUsed", "on_demand_used")
	if !hasOnDemandUsed {
		onDemandUsed, hasOnDemandUsed = cliproxyXAICents(usageData, "onDemandUsed", "on_demand_used")
	}
	billingPeriodEnd := firstStringFromMap(config, "billingPeriodEnd", "billing_period_end")
	if billingPeriodEnd == "" {
		billingPeriodEnd = firstStringFromMap(billingCycle, "billingPeriodEnd", "billing_period_end", "end")
	}
	if !hasQuota && !hasUsed && !hasOnDemandCap && !hasOnDemandUsed && billingPeriodEnd == "" {
		return cliproxyUsageRefreshBody{}, false
	}
	return cliproxyUsageRefreshBody{
		UsedTokens:               usedTokens,
		Quota:                    quota,
		PlanType:                 planType,
		OnDemandCap:              onDemandCap,
		XAIOnDemandUsed:          onDemandUsed,
		XAIOnDemandUsedRefreshed: hasOnDemandUsed,
		BillingPeriodEndAt:       unixFromRFC3339(billingPeriodEnd),
	}, true
}

func cliproxyXAIPayload(body map[string]any) map[string]any {
	if config := mapFromMap(body, "config"); len(config) > 0 {
		return config
	}
	if result := mapFromMap(body, "result"); len(result) > 0 {
		return result
	}
	return body
}

func extractCliproxyXAIPlan(result *service.CliproxyAPICallResponse) string {
	if result == nil {
		return ""
	}
	body := result.Body
	if len(body) == 0 {
		body = result.Data
	}
	return resolveCliproxyXAIPlan(body)
}

func resolveCliproxyXAIPlan(body map[string]any) string {
	config := cliproxyXAIPayload(body)
	sources := []map[string]any{firstMapFromMap(config, "settings"), config, body}
	for _, source := range sources {
		rawPlan := firstCliproxyXAIPlanFromMap(
			source,
			"subscription_tier_display",
			"subscriptionTierDisplay",
			"plan_type_display",
			"planTypeDisplay",
		)
		if rawPlan != "" {
			return canonicalCliproxyXAIPlan(rawPlan)
		}
	}
	for _, source := range sources {
		rawPlan := firstCliproxyXAIPlanFromMap(
			source,
			"subscription_tier",
			"subscriptionTier",
			"plan_type",
			"planType",
		)
		if rawPlan != "" {
			return canonicalCliproxyXAIPlan(rawPlan)
		}
	}
	return ""
}

func firstCliproxyXAIPlanFromMap(data map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := data[key].(string)
		if !ok {
			continue
		}
		if plan := strings.TrimSpace(value); plan != "" {
			return plan
		}
	}
	return ""
}

func canonicalCliproxyXAIPlan(rawPlan string) string {
	trimmedPlan := strings.TrimSpace(rawPlan)
	if trimmedPlan == "" {
		return ""
	}
	normalizedPlan := normalizeCliproxyPlan(trimmedPlan)
	normalizedPlan = strings.TrimPrefix(normalizedPlan, "subscriptiontier")
	switch normalizedPlan {
	case "free", "xaifree":
		return "Free"
	case "supergroklite":
		return "SuperGrok Lite"
	case "supergrok":
		return "SuperGrok"
	case "supergrokplus":
		return "SuperGrok Plus"
	case "supergrokheavy":
		return "SuperGrok Heavy"
	case "xpremium+", "xpremiumplus", "premium+", "premiumplus":
		return "X Premium+"
	default:
		return trimmedPlan
	}
}

func cliproxyXAICents(data map[string]any, keys ...string) (int, bool) {
	for _, key := range keys {
		value, ok := data[key]
		if !ok {
			continue
		}
		if valueMap, ok := value.(map[string]any); ok {
			return firstIntFromMap(valueMap, "val", "value"), true
		}
		return int(numberFromValue(value)), true
	}
	return 0, false
}

func classifyCliproxyUsageWindows(primaryWindow map[string]any, secondaryWindow map[string]any) (map[string]any, map[string]any) {
	windows := []map[string]any{}
	if len(primaryWindow) > 0 {
		windows = append(windows, primaryWindow)
	}
	if len(secondaryWindow) > 0 {
		windows = append(windows, secondaryWindow)
	}
	var fiveHourWindow map[string]any
	var weeklyWindow map[string]any
	for _, window := range windows {
		if int64FromMap(window, "limit_window_seconds") >= int64(24*60*60) {
			weeklyWindow = window
			continue
		}
		fiveHourWindow = window
	}
	return fiveHourWindow, weeklyWindow
}

func percentFromWindow(data map[string]any) int {
	percent := float64FromMap(data, "used_percent")
	if percent == 0 {
		percent = float64FromMap(data, "utilization")
	}
	if percent == 0 {
		percent = float64FromMap(data, "percent")
	}
	return int(math.Round(percent))
}

func resetAtFromWindow(data map[string]any) int64 {
	for _, key := range []string{"reset_at", "resets_at", "resetAt", "resetsAt"} {
		rawValue, ok := data[key]
		if !ok {
			continue
		}
		if value := int64(numberFromValue(rawValue)); value > 0 {
			if value >= 1_000_000_000_000 {
				return value / 1000
			}
			return value
		}
		raw := strings.TrimSpace(fmt.Sprintf("%v", rawValue))
		if parsed := unixFromRFC3339(raw); parsed > 0 {
			return parsed
		}
	}
	return 0
}

func hasCliproxyUsageWindowData(data map[string]any) bool {
	if resetAtFromWindow(data) > 0 {
		return true
	}
	for _, key := range []string{"used_percent", "utilization", "percent"} {
		if _, ok := data[key]; ok {
			return true
		}
	}
	return false
}

func isCliproxyClaudeAuthFile(binding *model.CliproxyAuthFileBinding) bool {
	if binding == nil {
		return false
	}
	if isCliproxyClaudePlanType(binding.LastPlanType) {
		return true
	}
	return isCliproxyClaudeAuthFileName(binding.AuthFile) || isCliproxyClaudeAuthFileName(binding.AuthName)
}

func isCliproxyXAIAuthFile(binding *model.CliproxyAuthFileBinding) bool {
	if binding == nil {
		return false
	}
	switch normalizeCliproxyPlan(binding.LastPlanType) {
	case "xai", "xaifree", "supergroklite", "supergrok", "supergrokplus", "supergrokheavy", "xpremium+", "xpremiumplus", "subscriptiontiersupergroklite", "subscriptiontiersupergrok", "subscriptiontiersupergrokplus", "subscriptiontiersupergrokheavy":
		return true
	}
	return isCliproxyXAIAuthFileName(binding.AuthFile) || isCliproxyXAIAuthFileName(binding.AuthName)
}

func isCliproxyClaudePlanType(value string) bool {
	switch normalizeCliproxyPlan(value) {
	case "claude", "planmax", "claudemax", "claudemax5x", "claudemax20x", "defaultclaudemax5x", "defaultclaudemax20x", "planpro", "claudepro", "planteam", "claudeteam", "planfree", "claudefree", "claudeenterprise":
		return true
	default:
		return false
	}
}

func isCliproxyXAIAuthFileName(value string) bool {
	return hasCliproxyAuthFileNamePrefix(value, "xai")
}

func isCliproxyClaudeAuthFileName(value string) bool {
	return hasCliproxyAuthFileNamePrefix(value, "claude")
}

func hasCliproxyAuthFileNamePrefix(value string, prefix string) bool {
	normalized := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(value)), "\\", "/")
	if normalized == "" {
		return false
	}
	segments := strings.Split(normalized, "/")
	name := segments[len(segments)-1]
	return strings.HasPrefix(name, prefix+"-") || strings.HasPrefix(name, prefix+"_")
}

func resolveCliproxyClaudeProfilePlan(profile map[string]any) string {
	if len(profile) == 0 {
		return ""
	}
	organization := mapFromMap(profile, "organization")
	organizationType := normalizeCliproxyPlan(stringFromMap(organization, "organization_type"))
	rateLimitTier := normalizeCliproxyPlan(stringFromMap(organization, "rate_limit_tier"))
	subscriptionStatus := strings.ToLower(stringFromMap(organization, "subscription_status"))
	switch organizationType {
	case "claudemax":
		switch rateLimitTier {
		case "defaultclaudemax5x", "claudemax5x", "max5x":
			return service.CliproxyClaudePlanMax5x
		case "defaultclaudemax20x", "claudemax20x", "max20x":
			return service.CliproxyClaudePlanMax20x
		default:
			return service.CliproxyClaudePlanMax
		}
	case "claudepro":
		return service.CliproxyClaudePlanPro
	case "claudeteam":
		if subscriptionStatus == "" || subscriptionStatus == "active" {
			return service.CliproxyClaudePlanTeam
		}
	case "claudeenterprise":
		return service.CliproxyClaudePlanEnterprise
	case "claudefree":
		return service.CliproxyClaudePlanFree
	}

	switch rateLimitTier {
	case "defaultclaudemax5x", "claudemax5x", "max5x":
		return service.CliproxyClaudePlanMax5x
	case "defaultclaudemax20x", "claudemax20x", "max20x":
		return service.CliproxyClaudePlanMax20x
	}

	account := mapFromMap(profile, "account")
	if value, ok := boolFromMap(account, "has_claude_max"); ok && value {
		return service.CliproxyClaudePlanMax
	}
	if value, ok := boolFromMap(account, "has_claude_pro"); ok && value {
		return service.CliproxyClaudePlanPro
	}
	hasMax, hasMaxOK := boolFromMap(account, "has_claude_max")
	hasPro, hasProOK := boolFromMap(account, "has_claude_pro")
	if hasMaxOK && hasProOK && !hasMax && !hasPro {
		return service.CliproxyClaudePlanFree
	}
	return ""
}

func mapFromMap(data map[string]any, key string) map[string]any {
	value, ok := data[key]
	if !ok {
		return nil
	}
	typedValue, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	return typedValue
}

func firstMapFromMap(data map[string]any, keys ...string) map[string]any {
	for _, key := range keys {
		if value := mapFromMap(data, key); len(value) > 0 {
			return value
		}
	}
	return nil
}

func boolFromMap(data map[string]any, key string) (bool, bool) {
	value, ok := data[key]
	if !ok {
		return false, false
	}
	switch typedValue := value.(type) {
	case bool:
		return typedValue, true
	case float64:
		return typedValue != 0, true
	case int:
		return typedValue != 0, true
	case string:
		parsedValue, err := strconv.ParseBool(strings.TrimSpace(typedValue))
		if err != nil {
			return false, false
		}
		return parsedValue, true
	default:
		return false, false
	}
}

func sliceFromMap(data map[string]any, key string) []any {
	value, ok := data[key]
	if !ok {
		return nil
	}
	typedValue, ok := value.([]any)
	if !ok {
		return nil
	}
	return typedValue
}

func firstSliceFromMap(data map[string]any, keys ...string) []any {
	for _, key := range keys {
		if value := sliceFromMap(data, key); value != nil {
			return value
		}
	}
	return nil
}

func stringFromMap(data map[string]any, key string) string {
	value, ok := data[key]
	if !ok {
		return ""
	}
	return strings.TrimSpace(fmt.Sprintf("%v", value))
}

func firstStringFromMap(data map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := stringFromMap(data, key); value != "" {
			return value
		}
	}
	return ""
}

func int64FromMap(data map[string]any, key string) int64 {
	return int64(float64FromMap(data, key))
}

func unixFromRFC3339(raw string) int64 {
	if raw == "" {
		return 0
	}
	if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
		return parsed.Unix()
	}
	if parsed, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return parsed.Unix()
	}
	return 0
}

func float64FromMap(data map[string]any, key string) float64 {
	value, ok := data[key]
	if !ok {
		return 0
	}
	return numberFromValue(value)
}

func numberFromValue(value any) float64 {
	switch typedValue := value.(type) {
	case float64:
		return typedValue
	case int:
		return float64(typedValue)
	case int64:
		return float64(typedValue)
	case string:
		parsedValue, _ := strconv.ParseFloat(strings.TrimSpace(typedValue), 64)
		return parsedValue
	default:
		return 0
	}
}

func intFromMap(data map[string]any, key string) int {
	return int(float64FromMap(data, key))
}

func firstIntFromMap(data map[string]any, keys ...string) int {
	for _, key := range keys {
		if _, ok := data[key]; ok {
			return intFromMap(data, key)
		}
	}
	return 0
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func normalizeCliproxyPlan(value string) string {
	return strings.NewReplacer("-", "", "_", "", " ", "").Replace(strings.ToLower(strings.TrimSpace(value)))
}

func parseOptionalBool(raw string) *bool {
	trimmedRaw := strings.TrimSpace(raw)
	if trimmedRaw == "" {
		return nil
	}
	parsedValue, err := strconv.ParseBool(trimmedRaw)
	if err != nil {
		return nil
	}
	return &parsedValue
}

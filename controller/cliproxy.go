package controller

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

const cliproxyWhamUsageURL = "https://chatgpt.com/backend-api/wham/usage"

type cliproxyAuthFileBindingRequest struct {
	UserId      int    `json:"user_id"`
	AuthIndex   string `json:"auth_index"`
	AuthName    string `json:"auth_name"`
	AuthFile    string `json:"auth_file"`
	Description string `json:"description"`
	AccountId   string `json:"account_id"`
	Enabled     bool   `json:"enabled"`
}

type cliproxyUsageRefreshBody struct {
	UsedTokens           int
	Quota                int
	PlanType             string
	FiveHourPercent      int
	FiveHourResetAt      int64
	WeeklyPercent        int
	WeeklyResetAt        int64
	CodexFiveHourPercent int
	CodexFiveHourResetAt int64
	CodexWeeklyPercent   int
	CodexWeeklyResetAt   int64
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
	query := model.CliproxyAuthFileBindingQuery{
		Username:  c.Query("username"),
		AuthIndex: c.Query("auth_index"),
		Enabled:   parseOptionalBool(c.Query("enabled")),
	}
	bindings, total, err := model.GetCliproxyAuthFileBindings(query, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(bindings)
	common.ApiSuccess(c, pageInfo)
}

func CreateCliproxyAuthFileBinding(c *gin.Context) {
	update, err := decodeCliproxyAuthFileBindingRequest(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	binding := &model.CliproxyAuthFileBinding{
		UserId:      update.UserId,
		Username:    update.Username,
		AuthIndex:   update.AuthIndex,
		AuthName:    update.AuthName,
		AuthFile:    update.AuthFile,
		Description: update.Description,
		AccountId:   update.AccountId,
		Enabled:     update.Enabled,
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
	client, err := newCliproxyClientFromOptions()
	if err != nil {
		common.ApiError(c, err)
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
	updatedBinding, err := model.UpdateCliproxyAuthFileBindingUsage(id, model.CliproxyUsageRefreshUpdate{
		LastUsageTokens:          usage.UsedTokens,
		LastUsageQuota:           usage.Quota,
		LastPlanType:             usage.PlanType,
		LastFiveHourPercent:      usage.FiveHourPercent,
		LastFiveHourResetAt:      usage.FiveHourResetAt,
		LastWeeklyPercent:        usage.WeeklyPercent,
		LastWeeklyResetAt:        usage.WeeklyResetAt,
		LastCodexFiveHourPercent: usage.CodexFiveHourPercent,
		LastCodexFiveHourResetAt: usage.CodexFiveHourResetAt,
		LastCodexWeeklyPercent:   usage.CodexWeeklyPercent,
		LastCodexWeeklyResetAt:   usage.CodexWeeklyResetAt,
		LastError:                "",
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
	query := model.UserTokenUsageQuery{
		StartTimestamp: startTimestamp,
		EndTimestamp:   endTimestamp,
		Username:       c.Query("username"),
		TokenName:      c.Query("token_name"),
		AuthIndex:      c.Query("auth_index"),
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
		UserId:      user.Id,
		Username:    user.Username,
		AuthIndex:   strings.TrimSpace(request.AuthIndex),
		AuthName:    strings.TrimSpace(request.AuthName),
		AuthFile:    request.AuthFile,
		Description: request.Description,
		AccountId:   strings.TrimSpace(request.AccountId),
		Enabled:     request.Enabled,
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
	fiveHourWindow, weeklyWindow := resolveCliproxyUsageWindows(body)
	if len(fiveHourWindow) == 0 && len(weeklyWindow) == 0 && stringFromMap(body, "plan_type") == "" {
		return cliproxyUsageRefreshBody{}, fmt.Errorf("刷新结果格式无效")
	}
	codexFiveHourWindow, codexWeeklyWindow := resolveCliproxyCodexUsageWindows(body)
	return cliproxyUsageRefreshBody{
		UsedTokens:           intFromMap(body, "used_tokens"),
		Quota:                intFromMap(body, "quota"),
		PlanType:             stringFromMap(body, "plan_type"),
		FiveHourPercent:      percentFromWindow(fiveHourWindow),
		FiveHourResetAt:      int64FromMap(fiveHourWindow, "reset_at"),
		WeeklyPercent:        percentFromWindow(weeklyWindow),
		WeeklyResetAt:        int64FromMap(weeklyWindow, "reset_at"),
		CodexFiveHourPercent: percentFromWindow(codexFiveHourWindow),
		CodexFiveHourResetAt: int64FromMap(codexFiveHourWindow, "reset_at"),
		CodexWeeklyPercent:   percentFromWindow(codexWeeklyWindow),
		CodexWeeklyResetAt:   int64FromMap(codexWeeklyWindow, "reset_at"),
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
	return int(math.Round(float64FromMap(data, "used_percent")))
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

func stringFromMap(data map[string]any, key string) string {
	value, ok := data[key]
	if !ok {
		return ""
	}
	return strings.TrimSpace(fmt.Sprintf("%v", value))
}

func int64FromMap(data map[string]any, key string) int64 {
	return int64(float64FromMap(data, key))
}

func float64FromMap(data map[string]any, key string) float64 {
	value, ok := data[key]
	if !ok {
		return 0
	}
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

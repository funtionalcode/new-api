package controller

import (
	"fmt"
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
	UsedTokens int `json:"used_tokens"`
	Quota      int `json:"quota"`
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
	usage := extractCliproxyUsage(result)
	updatedBinding, err := model.UpdateCliproxyAuthFileBindingUsage(id, model.CliproxyUsageRefreshUpdate{
		LastUsageTokens: usage.UsedTokens,
		LastUsageQuota:  usage.Quota,
		LastError:       "",
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

func extractCliproxyUsage(result *service.CliproxyAPICallResponse) cliproxyUsageRefreshBody {
	if result == nil {
		return cliproxyUsageRefreshBody{}
	}
	body := result.Body
	if len(body) == 0 {
		body = result.Data
	}
	return cliproxyUsageRefreshBody{
		UsedTokens: intFromMap(body, "used_tokens"),
		Quota:      intFromMap(body, "quota"),
	}
}

func intFromMap(data map[string]any, key string) int {
	value, ok := data[key]
	if !ok {
		return 0
	}
	switch typedValue := value.(type) {
	case float64:
		return int(typedValue)
	case int:
		return typedValue
	case int64:
		return int(typedValue)
	case string:
		parsedValue, _ := strconv.Atoi(strings.TrimSpace(typedValue))
		return parsedValue
	default:
		return 0
	}
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

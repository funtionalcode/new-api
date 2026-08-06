package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func useQuotaProxyTestDB(t *testing.T) {
	t.Helper()
	originalDB := DB
	t.Cleanup(func() {
		DB = originalDB
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&GLMQuotaBinding{}, &DeepSeekQuotaBinding{}, &KimiQuotaBinding{}))
	DB = db
}

func TestUpdateGLMQuotaBindingSavesAndClearsProxy(t *testing.T) {
	useQuotaProxyTestDB(t)

	binding := &GLMQuotaBinding{
		Name:                "glm",
		RequestCurl:         "curl https://bigmodel.cn",
		PlanType:            "标准版",
		FiveHourLimitTokens: 1,
		WeeklyLimitTokens:   1,
		Enabled:             true,
	}
	require.NoError(t, CreateGLMQuotaBinding(binding))

	updated, err := UpdateGLMQuotaBinding(binding.Id, GLMQuotaBindingUpdate{
		Name:                "glm",
		RequestCurl:         "curl ignored",
		Proxy:               "socks5://user:pass@example.com:1080",
		UpdateProxy:         true,
		PlanType:            "标准版",
		FiveHourLimitTokens: 1,
		WeeklyLimitTokens:   1,
		Enabled:             true,
	})
	require.NoError(t, err)
	require.Equal(t, "socks5://user:pass@example.com:1080", updated.Proxy)
	require.Equal(t, "curl https://bigmodel.cn", updated.RequestCurl)

	updated, err = UpdateGLMQuotaBinding(binding.Id, GLMQuotaBindingUpdate{
		Name:                "glm",
		Proxy:               "",
		UpdateProxy:         true,
		PlanType:            "标准版",
		FiveHourLimitTokens: 1,
		WeeklyLimitTokens:   1,
		Enabled:             true,
	})
	require.NoError(t, err)
	require.Empty(t, updated.Proxy)
}

func TestUpdateDeepSeekQuotaBindingSavesAndClearsProxy(t *testing.T) {
	useQuotaProxyTestDB(t)

	binding := &DeepSeekQuotaBinding{
		Name:        "deepseek",
		RequestCurl: "curl https://platform.deepseek.com",
		Enabled:     true,
	}
	require.NoError(t, CreateDeepSeekQuotaBinding(binding))

	updated, err := UpdateDeepSeekQuotaBinding(binding.Id, DeepSeekQuotaBindingUpdate{
		Name:        "deepseek",
		RequestCurl: "curl ignored",
		Proxy:       "http://proxy.example.com:8080",
		UpdateProxy: true,
		Enabled:     true,
	})
	require.NoError(t, err)
	require.Equal(t, "http://proxy.example.com:8080", updated.Proxy)
	require.Equal(t, "curl https://platform.deepseek.com", updated.RequestCurl)

	updated, err = UpdateDeepSeekQuotaBinding(binding.Id, DeepSeekQuotaBindingUpdate{
		Name:        "deepseek",
		Proxy:       "",
		UpdateProxy: true,
		Enabled:     true,
	})
	require.NoError(t, err)
	require.Empty(t, updated.Proxy)
}

func TestUpdateDeepSeekQuotaBindingSavesThreeCurlCommandsIndependently(t *testing.T) {
	useQuotaProxyTestDB(t)

	binding := &DeepSeekQuotaBinding{
		Name:            "deepseek",
		RequestCurl:     "curl https://platform.deepseek.com/summary",
		UsageAmountCurl: "curl https://platform.deepseek.com/amount",
		UsageCostCurl:   "curl https://platform.deepseek.com/cost",
		Enabled:         true,
	}
	require.NoError(t, CreateDeepSeekQuotaBinding(binding))

	updated, err := UpdateDeepSeekQuotaBinding(binding.Id, DeepSeekQuotaBindingUpdate{
		Name:                  "deepseek",
		UsageAmountCurl:       "curl https://platform.deepseek.com/amount-new",
		UpdateUsageAmountCurl: true,
		Enabled:               true,
	})
	require.NoError(t, err)
	require.Equal(t, binding.RequestCurl, updated.RequestCurl)
	require.Equal(t, "curl https://platform.deepseek.com/amount-new", updated.UsageAmountCurl)
	require.Equal(t, binding.UsageCostCurl, updated.UsageCostCurl)
	require.True(t, updated.HasCurl)
	require.True(t, updated.HasUsageAmountCurl)
	require.True(t, updated.HasUsageCostCurl)
}

func TestValidateDeepSeekQuotaBindingRequiresThreeCurlCommandsOnCreate(t *testing.T) {
	valid := DeepSeekQuotaBindingUpdate{
		Name:            "deepseek",
		RequestCurl:     "curl summary",
		UsageAmountCurl: "curl amount",
		UsageCostCurl:   "curl cost",
	}
	require.NoError(t, ValidateDeepSeekQuotaBindingUpdate(valid, true))

	missingAmount := valid
	missingAmount.UsageAmountCurl = ""
	require.EqualError(t, ValidateDeepSeekQuotaBindingUpdate(missingAmount, true), "DeepSeek 用量统计 curl 不能为空")

	missingCost := valid
	missingCost.UsageCostCurl = ""
	require.EqualError(t, ValidateDeepSeekQuotaBindingUpdate(missingCost, true), "DeepSeek 消费统计 curl 不能为空")
}

func TestUpdateDeepSeekQuotaBindingRejectsPartialUsageCurlConfiguration(t *testing.T) {
	useQuotaProxyTestDB(t)

	binding := &DeepSeekQuotaBinding{
		Name:        "legacy-deepseek",
		RequestCurl: "curl https://platform.deepseek.com/summary",
		Enabled:     true,
	}
	require.NoError(t, CreateDeepSeekQuotaBinding(binding))

	_, err := UpdateDeepSeekQuotaBinding(binding.Id, DeepSeekQuotaBindingUpdate{
		Name:                  binding.Name,
		UsageAmountCurl:       "curl https://platform.deepseek.com/amount",
		UpdateUsageAmountCurl: true,
		Enabled:               true,
	})
	require.EqualError(t, err, "DeepSeek 用量统计和消费统计 curl 必须同时配置")

	unchanged, err := GetDeepSeekQuotaBindingById(binding.Id)
	require.NoError(t, err)
	require.Empty(t, unchanged.UsageAmountCurl)
	require.Empty(t, unchanged.UsageCostCurl)
}

func TestUpdateKimiQuotaBindingSavesAndClearsProxy(t *testing.T) {
	useQuotaProxyTestDB(t)

	binding := &KimiQuotaBinding{
		Name:        "kimi",
		RequestCurl: "curl https://platform.kimi.com/api",
		Enabled:     true,
	}
	require.NoError(t, CreateKimiQuotaBinding(binding))

	updated, err := UpdateKimiQuotaBinding(binding.Id, KimiQuotaBindingUpdate{
		Name:        "kimi",
		RequestCurl: "curl ignored",
		Proxy:       "http://proxy.example.com:8080",
		UpdateProxy: true,
		Enabled:     true,
	})
	require.NoError(t, err)
	require.Equal(t, "http://proxy.example.com:8080", updated.Proxy)
	require.Equal(t, "curl https://platform.kimi.com/api", updated.RequestCurl)

	updated, err = UpdateKimiQuotaBinding(binding.Id, KimiQuotaBindingUpdate{
		Name:        "kimi",
		Proxy:       "",
		UpdateProxy: true,
		Enabled:     true,
	})
	require.NoError(t, err)
	require.Empty(t, updated.Proxy)
}

func TestUpdateKimiQuotaBindingSavesRefreshToken(t *testing.T) {
	useQuotaProxyTestDB(t)

	binding := &KimiQuotaBinding{
		Name:        "kimi",
		RequestCurl: "curl https://platform.kimi.com/api",
		Enabled:     true,
	}
	require.NoError(t, CreateKimiQuotaBinding(binding))

	updated, err := UpdateKimiQuotaBinding(binding.Id, KimiQuotaBindingUpdate{
		Name:               "kimi",
		RefreshToken:       "refresh-token-value",
		UpdateRefreshToken: true,
		Enabled:            true,
	})
	require.NoError(t, err)
	require.Equal(t, "refresh-token-value", updated.RefreshToken)
	require.True(t, updated.HasRefreshToken)

	updated, err = UpdateKimiQuotaBindingCredentials(binding.Id, KimiQuotaCredentialUpdate{
		RequestCurl:  "curl https://platform.kimi.com/api -H 'authorization: Bearer new-access'",
		RefreshToken: "rotated-refresh",
	})
	require.NoError(t, err)
	require.Equal(t, "rotated-refresh", updated.RefreshToken)
	require.Contains(t, updated.RequestCurl, "Bearer new-access")
}

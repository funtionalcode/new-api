package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMaskSensitiveInfoPreservesDomains(t *testing.T) {
	input := `status_code=500, Post "https://api.openai.com/v1/responses?api_key=secret&token=abc" EOF`

	result := MaskSensitiveInfo(input)

	require.Equal(t, `status_code=500, Post "https://api.openai.com/***/***?api_key=***&token=***" EOF`, result)
}

func TestMaskSensitiveInfoPreservesPlainDomains(t *testing.T) {
	result := MaskSensitiveInfo("upstream api.openai.com returned EOF")

	require.Equal(t, "upstream api.openai.com returned EOF", result)
}

func TestMaskSensitiveInfoStillMasksApiKeys(t *testing.T) {
	result := MaskSensitiveInfo(`header api_key:sk-test-secret`)

	require.Equal(t, `header api_key:***`, result)
}

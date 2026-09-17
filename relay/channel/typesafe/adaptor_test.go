package typesafe

import (
	"fmt"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTypeSafeUsageRejectsInvalidBillingData(t *testing.T) {
	for _, tc := range []struct {
		input, output string
		valid         bool
	}{
		{"0", "0", true}, {"312", "48", true}, {"-1", "48", false}, {"1", "-2", false},
		{"2147483647", "1", false}, {"18446744073709551615", "0", false}, {"null", "1", false},
	} {
		body := fmt.Sprintf(`{"model":"jev-latest","answers":{"ok":{"type":"noul","noul":0.9}},"usage":{"input_tokens":%s,"output_tokens":%s}}`, tc.input, tc.output)
		usage, err := ParseUsage([]byte(body))
		if tc.valid {
			require.NoError(t, err)
			assert.GreaterOrEqual(t, usage.TotalTokens, 0)
		} else {
			require.Error(t, err)
		}
	}
}

func TestTypeSafeEndpointDoesNotAcceptChat(t *testing.T) {
	a := &Adaptor{}
	for _, base := range []string{"https://api.typesafe.ai", "https://api.typesafe.ai/v1/"} {
		url, err := a.GetRequestURL(&relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeTypeSafe, ChannelMeta: &relaycommon.ChannelMeta{ChannelBaseUrl: base}})
		require.NoError(t, err)
		assert.Equal(t, "https://api.typesafe.ai/v1/systemone", url)
	}
	_, err := a.GetRequestURL(&relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeChatCompletions})
	assert.ErrorContains(t, err, "only supports")
}

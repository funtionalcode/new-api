package common

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/require"
)

func TestChannelType2APITypeForMimo(t *testing.T) {
	t.Parallel()

	apiType, ok := ChannelType2APIType(constant.ChannelTypeMimo)

	require.True(t, ok)
	require.Equal(t, constant.APITypeMimo, apiType)
}

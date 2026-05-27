package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay/channel/mimo"
	"github.com/stretchr/testify/require"
)

func TestGetAdaptorForMimo(t *testing.T) {
	t.Parallel()

	adaptor := GetAdaptor(constant.APITypeMimo)

	require.IsType(t, &mimo.Adaptor{}, adaptor)
}

package model

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/require"
)

func TestChannelGetKeysNormalizesLineDelimitedCredentials(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		channel Channel
	}{
		{
			name: "database value",
			channel: Channel{
				Key: "\r\n cursor-key-one \r\n\r\n\tcursor-key-two\t\r\n",
			},
		},
		{
			name: "memory cache",
			channel: Channel{
				Key:  "persisted-key-list",
				Keys: []string{" cursor-key-one\r ", "", "\tcursor-key-two\t"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, []string{"cursor-key-one", "cursor-key-two"}, test.channel.GetKeys())
		})
	}
}

func TestChannelGetNextEnabledKeyTrimsSingleCredential(t *testing.T) {
	t.Parallel()

	channel := &Channel{
		Key: "\r\n cursor-secret \r\n",
		ChannelInfo: ChannelInfo{
			MultiKeyMode: constant.MultiKeyModeRandom,
		},
	}

	key, index, apiErr := channel.GetNextEnabledKey()

	require.Nil(t, apiErr)
	require.Equal(t, "cursor-secret", key)
	require.Zero(t, index)
}

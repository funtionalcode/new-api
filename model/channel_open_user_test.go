package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestChannelOpenUserIdsNormalizeAndAccess(t *testing.T) {
	channel := &Channel{OpenUserIds: ChannelOpenUserIds{3, 1, 3, 0, -1}}

	require.Equal(t, []int{1, 3}, channel.GetOpenUserIds())
	require.True(t, channel.IsOpenToUser(1))
	require.True(t, channel.IsOpenToUser(3))
	require.False(t, channel.IsOpenToUser(2))
	require.False(t, channel.IsOpenToUser(0))
	require.True(t, (&Channel{}).IsOpenToUser(0))
}

func TestGetChannelForUserSkipsRestrictedHigherPriorityChannel(t *testing.T) {
	truncateTables(t)
	seedOpenUserChannel(t, 1001, "restricted", 20, ChannelOpenUserIds{2})
	seedOpenUserChannel(t, 1002, "open", 10, nil)

	channel, err := GetChannelForUser("default", "gpt-5", 0, 1)
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, 1002, channel.Id)

	channel, err = GetChannelForUser("default", "gpt-5", 0, 2)
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, 1001, channel.Id)
}

func TestGetRandomSatisfiedChannelForUserSkipsRestrictedHigherPriorityCache(t *testing.T) {
	truncateTables(t)
	seedOpenUserChannel(t, 2001, "restricted", 20, ChannelOpenUserIds{2})
	seedOpenUserChannel(t, 2002, "open", 10, nil)

	previousMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	InitChannelCache()
	t.Cleanup(func() {
		common.MemoryCacheEnabled = previousMemoryCacheEnabled
		InitChannelCache()
	})

	channel, err := GetRandomSatisfiedChannelForUser("default", "gpt-5", 0, 1)
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, 2002, channel.Id)

	channel, err = GetRandomSatisfiedChannelForUser("default", "gpt-5", 0, 2)
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, 2001, channel.Id)
}

func TestChannelUpdateClearsOpenUserIds(t *testing.T) {
	truncateTables(t)
	channel := seedOpenUserChannel(t, 3001, "restricted", 20, ChannelOpenUserIds{1, 2})

	channel.OpenUserIds = ChannelOpenUserIds{}
	require.NoError(t, channel.Update())

	updated, err := GetChannelById(3001, true)
	require.NoError(t, err)
	require.Empty(t, updated.GetOpenUserIds())
	require.True(t, updated.IsOpenToUser(999))
}

func seedOpenUserChannel(t *testing.T, id int, name string, priority int64, openUserIds ChannelOpenUserIds) *Channel {
	t.Helper()
	weight := uint(0)
	channel := &Channel{
		Id:          id,
		Type:        1,
		Key:         "sk-test",
		Status:      common.ChannelStatusEnabled,
		Name:        name,
		Models:      "gpt-5",
		Group:       "default",
		Priority:    &priority,
		Weight:      &weight,
		OpenUserIds: openUserIds,
	}
	require.NoError(t, channel.Insert())
	return channel
}

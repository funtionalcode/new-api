package cursor

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCursorAgentSerialGateRejectsConcurrentAcquisitionUntilRelease(t *testing.T) {
	pool := cursorAgentSerialGatePool{gates: make(map[string]*cursorAgentSerialGate)}
	release, err := pool.acquire(context.Background(), "agent-session")
	require.NoError(t, err)

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = pool.acquire(canceled, "agent-session")
	require.ErrorIs(t, err, context.Canceled)

	pool.mutex.Lock()
	assert.Equal(t, 1, pool.gates["agent-session"].refs)
	pool.mutex.Unlock()

	release()
	pool.mutex.Lock()
	assert.Empty(t, pool.gates)
	pool.mutex.Unlock()
}

func TestCursorAgentSerialResponseBodyReleasesGateOnClose(t *testing.T) {
	pool := cursorAgentSerialGatePool{gates: make(map[string]*cursorAgentSerialGate)}
	release, err := pool.acquire(context.Background(), "agent-session")
	require.NoError(t, err)
	response := &http.Response{Body: io.NopCloser(strings.NewReader("done"))}
	transferCursorAgentSerialRelease(response, release)

	require.NoError(t, response.Body.Close())
	require.NoError(t, response.Body.Close())

	pool.mutex.Lock()
	assert.Empty(t, pool.gates)
	pool.mutex.Unlock()
}

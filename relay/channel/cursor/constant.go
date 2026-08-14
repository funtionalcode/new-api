package cursor

import (
	"errors"
	"time"
)

var errCursorLegacyAgentSignature = errors.New("cursor channel: legacy persistent agent signature must be renewed")
var errCursorAgentSignatureMismatch = errors.New("cursor channel: persistent agent signature is no longer valid")

const (
	ChannelName = "Cursor"

	cursorAgentIDMetadataKey      = "cursor_agent_id"
	cursorPersistentMetadataKey   = "cursor_persistent"
	cursorAgentIDContextKey       = "cursor_agent_id"
	cursorPersistentContextKey    = "cursor_persistent"
	cursorExternalToolsContextKey = "cursor_external_tools"
	cursorAgentIDInternalHeader   = "X-New-API-Cursor-Agent-ID"
	cursorRunIDInternalHeader     = "X-New-API-Cursor-Run-ID"
	cursorPersistentInternalKey   = "X-New-API-Cursor-Persistent"
	cursorClientStreamHeader      = "X-New-API-Cursor-Client-Stream"
	cursorSkipRemoteUsageHeader   = "X-New-API-Cursor-Skip-Remote-Usage"
	cursorAgentLifecycleHeader    = "X-New-API-Cursor-Agent-Lifecycle"
	cursorEventStreamContentType  = "application/x-cursor-event-stream"
	cursorAgentSignatureVersion   = "v2"
	cursorExternalToolAliasPrefix = "client_external_tool_"
	cursorCleanupCommand          = "清理会话agent"
	cursorRunPollInterval         = time.Second
	cursorRunPollTimeout          = 10 * time.Minute
)

var ModelList = []string{}

package cursor

const (
	ChannelName = "Cursor"

	cursorAgentIDHeader          = "X-Cursor-Agent-ID"
	cursorAgentSignatureHeader   = "X-Cursor-Agent-Signature"
	cursorPersistentHeader       = "X-Cursor-Persistent"
	cursorAgentIDMetadataKey     = "cursor_agent_id"
	cursorPersistentMetadataKey  = "cursor_persistent"
	cursorAgentIDContextKey      = "cursor_agent_id"
	cursorPersistentContextKey   = "cursor_persistent"
	cursorAgentIDInternalHeader  = "X-New-API-Cursor-Agent-ID"
	cursorRunIDInternalHeader    = "X-New-API-Cursor-Run-ID"
	cursorPersistentInternalKey  = "X-New-API-Cursor-Persistent"
	cursorClientStreamHeader     = "X-New-API-Cursor-Client-Stream"
	cursorSkipRemoteUsageHeader  = "X-New-API-Cursor-Skip-Remote-Usage"
	cursorEventStreamContentType = "application/x-cursor-event-stream"
	cursorAgentSignatureVersion  = "v1"
)

var ModelList = []string{}

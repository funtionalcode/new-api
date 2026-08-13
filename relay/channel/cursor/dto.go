package cursor

type cursorPrompt struct {
	Text string `json:"text"`
}

type cursorModelSelection struct {
	ID string `json:"id"`
}

type createAgentRequest struct {
	Prompt cursorPrompt         `json:"prompt"`
	Model  cursorModelSelection `json:"model"`
	Name   string               `json:"name,omitempty"`
}

type createRunRequest struct {
	Prompt cursorPrompt `json:"prompt"`
}

type deleteAgentRequest struct{}

type createAgentResponse struct {
	Agent struct {
		ID          string `json:"id"`
		LatestRunID string `json:"latestRunId"`
	} `json:"agent"`
	Run struct {
		ID string `json:"id"`
	} `json:"run"`
	LatestRunID string `json:"latestRunId"`
}

type createRunResponse struct {
	Run struct {
		ID string `json:"id"`
	} `json:"run"`
}

type cursorMetadata struct {
	AgentID    string
	Persistent bool
}

type cursorTranscriptMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type cursorEvent struct {
	Type string
	Data []byte
}

type cursorTextEvent struct {
	Text string `json:"text"`
}

type cursorResultEvent struct {
	RunID      string `json:"runId"`
	Status     string `json:"status"`
	Text       string `json:"text"`
	DurationMS int64  `json:"durationMs"`
}

type cursorErrorEvent struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type cursorUsage struct {
	InputTokens      int64 `json:"inputTokens"`
	OutputTokens     int64 `json:"outputTokens"`
	CacheWriteTokens int64 `json:"cacheWriteTokens"`
	CacheReadTokens  int64 `json:"cacheReadTokens"`
	TotalTokens      int64 `json:"totalTokens"`
}

type cursorUsageResponse struct {
	TotalUsage cursorUsage `json:"totalUsage"`
	Runs       []struct {
		ID    string      `json:"id"`
		Usage cursorUsage `json:"usage"`
	} `json:"runs"`
}

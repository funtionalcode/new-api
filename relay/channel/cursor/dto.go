package cursor

type cursorPrompt struct {
	Text string `json:"text"`
}

type cursorModelSelection struct {
	ID     string             `json:"id"`
	Params []cursorModelParam `json:"params,omitempty"`
}

type cursorModelParam struct {
	ID    string `json:"id"`
	Value string `json:"value"`
}

type cursorModelCatalogResponse struct {
	Items []cursorModelCatalogItem `json:"items"`
}

type cursorModelCatalogItem struct {
	ID         string                           `json:"id"`
	Aliases    []string                         `json:"aliases,omitempty"`
	Parameters []cursorModelParameterDefinition `json:"parameters,omitempty"`
	Variants   []cursorModelVariant             `json:"variants,omitempty"`
}

type cursorModelParameterDefinition struct {
	ID          string                      `json:"id"`
	DisplayName string                      `json:"displayName,omitempty"`
	Values      []cursorModelParameterValue `json:"values,omitempty"`
}

type cursorModelParameterValue struct {
	Value string `json:"value"`
}

type cursorModelVariant struct {
	Params    []cursorModelParam `json:"params"`
	IsDefault bool               `json:"isDefault,omitempty"`
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
	Role       string                     `json:"role"`
	Content    string                     `json:"content,omitempty"`
	Name       string                     `json:"name,omitempty"`
	ToolCallID string                     `json:"tool_call_id,omitempty"`
	ToolCalls  []cursorTranscriptToolCall `json:"tool_calls,omitempty"`
}

type cursorTranscriptToolCall struct {
	ID        string `json:"id"`
	Type      string `json:"type,omitempty"`
	Name      string `json:"name"`
	Arguments any    `json:"arguments"`
}

type cursorExternalToolEnvelope struct {
	ToolCalls []cursorExternalToolCall `json:"cursor_external_tool_calls"`
}

type cursorExternalToolCall struct {
	ID        string `json:"id,omitempty"`
	Name      string `json:"name"`
	Arguments any    `json:"arguments,omitempty"`
	Input     any    `json:"input,omitempty"`
}

type cursorExternalToolSpec struct {
	Kind      string
	Name      string
	Namespace string
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

type cursorToolCallEvent struct {
	CallID string `json:"callId"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Result any    `json:"result,omitempty"`
}

type cursorErrorEvent struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type cursorAPIErrorResponse struct {
	Error cursorErrorEvent `json:"error"`
}

type cursorRunResponse struct {
	ID         string `json:"id"`
	AgentID    string `json:"agentId"`
	Status     string `json:"status"`
	Result     string `json:"result"`
	DurationMS int64  `json:"durationMs"`
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

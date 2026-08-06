package dto

import (
	"testing"

	kitutil "github.com/QuantumNous/new-api/relaykit/relayconvert/kitutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClaudeRequestTokenCountIncludesDecodedTools(t *testing.T) {
	raw := []byte(`{
		"model":"claude-test",
		"messages":[{"role":"user","content":"count"}],
		"max_tokens":1,
		"tools":[
			{
				"name":"lookup",
				"description":"Look up an item in the workspace",
				"input_schema":{"type":"object","properties":{"query":{"type":"string"}}}
			},
			{
				"type":"web_search_20250305",
				"name":"web_search",
				"max_uses":5,
				"user_location":{"type":"approximate","timezone":"Asia/Shanghai"}
			}
		]
	}`)

	var request ClaudeRequest
	require.NoError(t, kitutil.Unmarshal(raw, &request))

	meta := request.GetTokenCountMeta()

	require.NotNil(t, meta)
	assert.Equal(t, 2, meta.ToolsCount)
	assert.Contains(t, meta.CombineText, "lookup")
	assert.Contains(t, meta.CombineText, "Look up an item in the workspace")
	assert.Contains(t, meta.CombineText, "query")
	assert.Contains(t, meta.CombineText, "web_search")
	assert.Contains(t, meta.CombineText, "Asia/Shanghai")
}

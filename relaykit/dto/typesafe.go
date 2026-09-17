package dto

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/relaykit/types"
)

type TypeSafeQuestion struct {
	Type         string          `json:"type"`
	Instructions json.RawMessage `json:"instructions"`
	Criteria     json.RawMessage `json:"criteria,omitempty"`
}

type TypeSafeRequest struct {
	Model     string                      `json:"model"`
	State     json.RawMessage             `json:"state"`
	Questions map[string]TypeSafeQuestion `json:"questions"`
}

func (r *TypeSafeRequest) GetTokenCountMeta() *types.TokenCountMeta {
	parts := []string{string(r.State)}
	for _, question := range r.Questions {
		parts = append(parts, string(question.Instructions), string(question.Criteria))
	}
	return &types.TokenCountMeta{TokenType: types.TokenTypeTokenizer, CombineText: strings.Join(parts, "\n")}
}

func (r *TypeSafeRequest) IsStream(*http.Request) bool { return false }

func (r *TypeSafeRequest) SetModelName(model string) { r.Model = model }

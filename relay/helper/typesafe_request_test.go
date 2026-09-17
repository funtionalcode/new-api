package helper

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTypeSafeRequestValidation(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		valid      bool
	}{
		{"mixed", `{"model":"jev-latest","state":{"message":"hello"},"questions":{"a":{"type":"noul","instructions":"True?"},"b":{"type":"choice","instructions":["Pick"],"criteria":{"yes":null,"no":"negative"}},"c":{"type":"score","instructions":"Quality","criteria":["low","high"]}}}`, true},
		{"missing_state", `{"model":"jev-latest","questions":{"a":{"type":"noul","instructions":"True?"}}}`, false},
		{"null_state", `{"model":"jev-latest","state":null,"questions":{"a":{"type":"noul","instructions":"True?"}}}`, false},
		{"no_questions", `{"model":"jev-latest","state":"hello","questions":{}}`, false},
		{"choice_without_options", `{"model":"jev-latest","state":"hello","questions":{"a":{"type":"choice","instructions":"Pick"}}}`, false},
		{"invalid_type", `{"model":"jev-latest","state":"hello","questions":{"a":{"type":"chat","instructions":"Talk"}}}`, false},
		{"invalid_score", `{"model":"jev-latest","state":"hello","questions":{"a":{"type":"score","instructions":"Quality","criteria":[10]}}}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var request dto.TypeSafeRequest
			require.NoError(t, common.UnmarshalJsonStr(tc.body, &request))
			err := ValidateTypeSafeRequest(&request)
			if tc.valid {
				assert.NoError(t, err)
				assert.Contains(t, request.GetTokenCountMeta().CombineText, "hello")
			} else {
				assert.Error(t, err)
			}
		})
	}
}

package types_test

import (
	"errors"
	"testing"

	relaytypes "github.com/QuantumNous/new-api/relaykit/types"
	"github.com/stretchr/testify/require"
)

func TestMaskSensitiveErrorWithStatusCodePreservesDomain(t *testing.T) {
	apiErr := relaytypes.NewErrorWithStatusCode(
		errors.New(`Post "https://api.openai.com/v1/responses": EOF`),
		relaytypes.ErrorCodeDoRequestFailed,
		500,
	)

	require.Equal(
		t,
		`status_code=500, Post "https://api.openai.com/***/***": EOF`,
		apiErr.MaskSensitiveErrorWithStatusCode(),
	)
}

func TestComposeUpstreamErrorMessageEnrichesGenericError(t *testing.T) {
	require.Equal(t, "model not allowed", relaytypes.ComposeUpstreamErrorMessage("model not allowed", "invalid_request_error", "model_not_found"))
	require.Equal(t, "Error (type=invalid_request_error, code=model_not_found)", relaytypes.ComposeUpstreamErrorMessage("Error", "invalid_request_error", "model_not_found"))
	require.Equal(t, "invalid_request_error", relaytypes.ComposeUpstreamErrorMessage("", "invalid_request_error", nil))
	require.Equal(t, "Error (code=model_not_found)", relaytypes.ComposeUpstreamErrorMessage("Error", "error", "model_not_found"))
}

func TestWithOpenAIErrorPreservesTypeAndCodeInGenericMessage(t *testing.T) {
	apiErr := relaytypes.WithOpenAIError(relaytypes.OpenAIError{
		Message: "Error",
		Type:    "invalid_request_error",
		Code:    "model_not_found",
	}, 500)
	require.Equal(t, "Error (type=invalid_request_error, code=model_not_found)", apiErr.Error())
	require.Equal(t, "status_code=500, Error (type=invalid_request_error, code=model_not_found)", apiErr.MaskSensitiveErrorWithStatusCode())
}

func TestWithClaudeErrorPreservesTypeInGenericMessage(t *testing.T) {
	apiErr := relaytypes.WithClaudeError(relaytypes.ClaudeError{
		Message: "Error",
		Type:    "invalid_request_error",
	}, 500)
	require.Equal(t, "Error (type=invalid_request_error)", apiErr.Error())
}

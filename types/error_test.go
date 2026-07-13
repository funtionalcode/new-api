package types

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMaskSensitiveErrorWithStatusCodePreservesDomain(t *testing.T) {
	apiErr := NewErrorWithStatusCode(
		errors.New(`Post "https://api.openai.com/v1/responses": EOF`),
		ErrorCodeDoRequestFailed,
		500,
	)

	require.Equal(
		t,
		`status_code=500, Post "https://api.openai.com/***/***": EOF`,
		apiErr.MaskSensitiveErrorWithStatusCode(),
	)
}

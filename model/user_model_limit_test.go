package model

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/assert"
)

func TestIsModelAllowedByUserLimitAllowsCompactVariantWhenBaseModelIsAllowed(t *testing.T) {
	limits := BuildUserModelLimitMap([]string{"gpt-5.6-sol"})

	assert.True(t, IsModelAllowedByUserLimit("gpt-5.6-sol", limits))
	assert.True(t, IsModelAllowedByUserLimit(ratio_setting.WithCompactModelSuffix("gpt-5.6-sol"), limits))
	assert.False(t, IsModelAllowedByUserLimit(ratio_setting.WithCompactModelSuffix("gpt-5.6-pro"), limits))
}

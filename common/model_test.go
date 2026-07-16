package common

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestXAIImageModelsUseImageGenerationEndpoint(t *testing.T) {
	tests := []string{
		"grok-imagine-image-quality",
		"grok-imagine-image-pro",
		"grok-imagine-image",
		"grok-2-image-1212",
	}

	for _, modelName := range tests {
		t.Run(modelName, func(t *testing.T) {
			assert.True(t, IsImageGenerationModel(modelName))

			endpoints := GetEndpointTypesByChannelType(constant.ChannelTypeXai, modelName)
			require.NotEmpty(t, endpoints)
			assert.Equal(t, constant.EndpointTypeImageGeneration, endpoints[0])
		})
	}
}

package common

import (
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/tidwall/sjson"
)

// ApplyAdaptiveReasoning 在固定参数覆盖后只修改推理强度，保持消息和缓存前缀不变。
func ApplyAdaptiveReasoning(data []byte, info *RelayInfo) ([]byte, error) {
	if info == nil || info.AdaptiveReasoningEffort == "" {
		return data, nil
	}
	path := "reasoning_effort"
	if info.RelayMode == relayconstant.RelayModeResponses {
		path = "reasoning.effort"
	}
	updated, err := sjson.SetBytes(data, path, info.AdaptiveReasoningEffort)
	if err != nil {
		return nil, err
	}
	info.SetReasoningEffort(info.AdaptiveReasoningEffort)
	if info.AdaptiveReasoningResult != nil {
		info.AdaptiveReasoningResult["applied"] = true
	}
	return updated, nil
}

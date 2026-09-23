package common

import (
	"slices"

	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	kitreasoning "github.com/QuantumNous/new-api/relaykit/relayconvert/reasoning"
	"github.com/QuantumNous/new-api/relaykit/types"
	hostreasoning "github.com/QuantumNous/new-api/setting/reasoning"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// AdaptiveClaudeEfforts 复用协议转换层的模型能力，排除不支持原生 effort 的旧模型和档位。
func AdaptiveClaudeEfforts(model string, configured []string) []string {
	var allowed []string
	for _, effort := range configured {
		if !slices.Contains([]string{"low", "medium", "high", "xhigh", "max"}, effort) {
			continue
		}
		render, err := kitreasoning.RenderClaude(model, kitreasoning.Intent{Mode: kitreasoning.ModeAdaptive, Effort: kitreasoning.Effort(effort)}, nil, 0)
		if err == nil && render.Thinking != nil && render.Thinking.Type == "adaptive" && string(render.OutputEffort) == effort {
			allowed = append(allowed, effort)
		}
	}
	return allowed
}

// ApplyAdaptiveReasoning 在固定参数覆盖后按最终协议应用推理配置，保留消息和缓存标记。
func ApplyAdaptiveReasoning(data []byte, info *RelayInfo) ([]byte, error) {
	if info == nil || info.AdaptiveReasoningEffort == "" {
		return data, nil
	}
	path := "reasoning_effort"
	format := info.GetFinalRequestRelayFormat()
	if format == types.RelayFormatOpenAIResponses || format == "" && info.RelayMode == relayconstant.RelayModeResponses {
		path = "reasoning.effort"
	}
	if format == types.RelayFormatClaude {
		modelName := hostreasoning.BaseModelName(gjson.GetBytes(data, "model").String())
		if len(AdaptiveClaudeEfforts(modelName, []string{info.AdaptiveReasoningEffort})) == 0 {
			if info.AdaptiveReasoningResult != nil {
				info.AdaptiveReasoningResult["status"], info.AdaptiveReasoningResult["reason"] = "skipped", "unsupported_reasoning_model"
			}
			return data, nil
		}
		path = "output_config.effort"
		render, err := kitreasoning.RenderClaude(modelName, kitreasoning.Intent{Mode: kitreasoning.ModeAdaptive, Effort: kitreasoning.Effort(info.AdaptiveReasoningEffort)}, nil, 0)
		if err != nil {
			return nil, err
		}
		data, err = sjson.SetBytes(data, "thinking.type", "adaptive")
		if err != nil {
			return nil, err
		}
		remove := []string{"thinking.budget_tokens"}
		if render.ClearSampling {
			remove = append(remove, "temperature", "top_p", "top_k")
		} else if render.ConstrainThinkingSampling {
			remove = append(remove, "top_k")
			for key, value := range map[string]float64{"temperature": 1, "top_p": 0.95} {
				current := gjson.GetBytes(data, key)
				if current.Exists() && (key == "temperature" || current.Float() < value) {
					data, err = sjson.SetBytes(data, key, value)
					if err != nil {
						return nil, err
					}
				}
			}
		}
		for _, key := range remove {
			data, err = sjson.DeleteBytes(data, key)
			if err != nil {
				return nil, err
			}
		}
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

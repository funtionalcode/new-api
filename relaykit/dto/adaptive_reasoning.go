package dto

import (
	"fmt"
	"slices"
	"strings"
)

// AdaptiveReasoningConfig 仅用于网关决策，不发送给主模型。
type AdaptiveReasoningConfig struct {
	Enabled             bool     `json:"enabled"`
	ChannelID           int      `json:"channel_id"`
	Model               string   `json:"model,omitempty"`
	Efforts             []string `json:"efforts,omitempty"`
	MaxReuseGenerations int      `json:"max_reuse_generations,omitempty"`
	TimeoutMS           int      `json:"timeout_ms,omitempty"`
	MaxChars            int      `json:"max_chars,omitempty"`
}

func (c AdaptiveReasoningConfig) WithDefaults() AdaptiveReasoningConfig {
	if c.Model == "" {
		c.Model = "jev-latest"
	}
	if len(c.Efforts) == 0 {
		c.Efforts = []string{"low", "medium", "high", "xhigh"}
	}
	if c.MaxReuseGenerations == 0 {
		c.MaxReuseGenerations = 10
	}
	if c.TimeoutMS == 0 {
		c.TimeoutMS = 1500
	}
	if c.MaxChars == 0 {
		c.MaxChars = 12000
	}
	return c
}

func (c *AdaptiveReasoningConfig) Validate() error {
	if c == nil || !c.Enabled {
		return nil
	}
	if c.ChannelID <= 0 {
		return fmt.Errorf("adaptive_reasoning.channel_id must be positive")
	}
	if len(c.Model) > 200 || c.Model != strings.TrimSpace(c.Model) {
		return fmt.Errorf("invalid adaptive_reasoning.model")
	}
	if !slices.Contains([]int{0, 1, 2, 5, 10}, c.MaxReuseGenerations) {
		return fmt.Errorf("adaptive_reasoning.max_reuse_generations must be 1, 2, 5 or 10")
	}
	if c.TimeoutMS < 0 || c.TimeoutMS > 30000 {
		return fmt.Errorf("adaptive_reasoning.timeout_ms must be between 0 and 30000")
	}
	if c.MaxChars < 0 || c.MaxChars > 60000 {
		return fmt.Errorf("adaptive_reasoning.max_chars must be between 0 and 60000")
	}
	seen := make(map[string]bool)
	for _, effort := range c.Efforts {
		if !slices.Contains([]string{"none", "minimal", "low", "medium", "high", "xhigh", "max"}, effort) || seen[effort] {
			return fmt.Errorf("invalid or duplicate adaptive_reasoning effort: %s", effort)
		}
		seen[effort] = true
	}
	return nil
}

package dto

import "fmt"

// TypeSafeIntegration 是渠道本地参数，不发往主模型供应商。
type TypeSafeIntegration struct {
	ChannelID int                         `json:"channel_id"`
	Model     string                      `json:"model,omitempty"`
	Before    map[string]TypeSafeQuestion `json:"before,omitempty"`
	After     map[string]TypeSafeQuestion `json:"after,omitempty"`
	TimeoutMS int                         `json:"timeout_ms,omitempty"`
	MaxChars  int                         `json:"max_chars,omitempty"`
}

func (c *TypeSafeIntegration) Validate() error {
	if c.ChannelID <= 0 {
		return fmt.Errorf("_typesafe.channel_id must be positive")
	}
	if len(c.Before) == 0 && len(c.After) == 0 {
		return fmt.Errorf("_typesafe requires before or after questions")
	}
	if c.TimeoutMS < 0 || c.TimeoutMS > 30000 {
		return fmt.Errorf("_typesafe.timeout_ms must be between 0 and 30000")
	}
	if c.MaxChars < 0 || c.MaxChars > 60000 {
		return fmt.Errorf("_typesafe.max_chars must be between 0 and 60000")
	}
	return nil
}

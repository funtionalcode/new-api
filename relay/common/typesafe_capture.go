package common

import (
	"bytes"
	"strings"

	"github.com/tidwall/gjson"
)

// TypeSafeCapture 只保留有限的公开回答文本，兼容 JSON、SSE 和 WebSocket。
type TypeSafeCapture struct {
	Limit      int
	Truncated  bool
	text       strings.Builder
	characters int
	pending    []byte
	stream     bool
}

func (c *TypeSafeCapture) append(text string) {
	for _, r := range text {
		if c.characters >= c.Limit {
			c.Truncated = true
			return
		}
		c.text.WriteRune(r)
		c.characters++
	}
}

func (c *TypeSafeCapture) ObserveJSON(data []byte) {
	if !gjson.ValidBytes(data) {
		return
	}
	value := gjson.ParseBytes(data)
	switch value.Get("type").String() {
	case "response.output_text.delta":
		c.append(value.Get("delta").String())
		return
	case "content_block_delta":
		c.append(value.Get("delta.text").String())
		return
	case "content_block_start":
		c.append(value.Get("content_block.text").String())
		return
	}
	for _, choice := range value.Get("choices").Array() {
		for _, path := range []string{"delta.content", "message.content", "text"} {
			content := choice.Get(path)
			if content.Type == gjson.String {
				c.append(content.String())
			}
			for _, part := range content.Array() {
				c.append(part.Get("text").String())
			}
		}
	}
	for _, block := range value.Get("content").Array() {
		if block.Get("type").String() == "text" {
			c.append(block.Get("text").String())
		}
	}
	for _, candidate := range value.Get("candidates").Array() {
		for _, part := range candidate.Get("content.parts").Array() {
			if !part.Get("thought").Bool() {
				c.append(part.Get("text").String())
			}
		}
	}
	// Responses 的完成事件包含完整副本；已有增量时不重复追加。
	if c.characters == 0 {
		output := value.Get("output")
		if !output.Exists() {
			output = value.Get("response.output")
		}
		for _, item := range output.Array() {
			if item.Get("type").String() == "message" {
				for _, part := range item.Get("content").Array() {
					c.append(part.Get("text").String())
				}
			}
		}
	}
}

func (c *TypeSafeCapture) Write(data []byte, streaming bool) {
	c.stream = streaming
	if len(c.pending)+len(data) > 2*1024*1024 {
		c.Truncated = true
		return
	}
	c.pending = append(c.pending, data...)
	if !streaming {
		return
	}
	for {
		end := bytes.IndexByte(c.pending, '\n')
		if end < 0 {
			return
		}
		line := bytes.TrimSpace(c.pending[:end])
		if bytes.HasPrefix(line, []byte("data:")) {
			c.ObserveJSON(bytes.TrimSpace(line[5:]))
		}
		c.pending = c.pending[end+1:]
	}
}

func (c *TypeSafeCapture) Text() string {
	if len(c.pending) > 0 {
		data := bytes.TrimSpace(c.pending)
		if c.stream {
			data = bytes.TrimSpace(bytes.TrimPrefix(data, []byte("data:")))
		}
		c.ObserveJSON(data)
		c.pending = nil
	}
	return c.text.String()
}

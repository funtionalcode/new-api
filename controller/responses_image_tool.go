package controller

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/relay"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type responsesImageToolContextKey struct{}

type responsesImageToolSubrequest struct {
	image bool
}

func isResponsesImageToolSubmission(c *gin.Context) bool {
	request, ok := c.Request.Context().Value(responsesImageToolContextKey{}).(responsesImageToolSubrequest)
	return ok && request.image
}

type responsesImageTool struct {
	request  []byte
	tools    common.RawMessage
	choice   common.RawMessage
	name     string
	settings gjson.Result
	allowed  bool
}

// ResponsesImageToolMiddleware 内部执行 image_gen.imagegen，实际生成仍经过完整图片路由。
func ResponsesImageToolMiddleware(dispatcher http.Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Context().Value(responsesImageToolContextKey{}) != nil {
			c.Next()
			return
		}
		storage, err := common.GetBodyStorage(c)
		if err != nil {
			c.Next()
			return
		}
		body, err := storage.Bytes()
		if err != nil || !hasResponsesImageTool(body) {
			c.Next()
			return
		}
		c.Abort()
		var write helper.ResponsesStreamWriter
		if gjson.GetBytes(body, "stream").Bool() {
			defer helper.StartStreamKeepAlive(c, helper.DefaultPingInterval)()
			write = func(data []byte) error {
				helper.SetEventStreamHeaders(c)
				return helper.ResponseChunkData(c, dto.ResponsesStreamResponse{Type: gjson.GetBytes(data, "type").String()}, string(data))
			}
		}
		response, apiErr := runResponsesImageTool(c, dispatcher, body, write)
		if apiErr != nil {
			if c.Writer.Written() {
				_ = writeResponsesImageToolError(write, apiErr)
			} else {
				c.Header("Content-Type", "application/json")
				c.JSON(apiErr.StatusCode, gin.H{"error": apiErr.ToOpenAIError()})
			}
			return
		}
		if write == nil {
			c.Data(http.StatusOK, "application/json", response)
		}
	}
}

// ResponsesWebSocketWithImageTools 为每条 response.create 复用相同的内部路由。
func ResponsesWebSocketWithImageTools(dispatcher http.Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		responsesWebSocket(c, &relay.ResponsesWSImageToolHandler{
			Match: hasResponsesImageTool,
			Run: func(requestContext *gin.Context, body []byte, write helper.ResponsesStreamWriter) *types.NewAPIError {
				_, apiErr := runResponsesImageTool(requestContext, dispatcher, body, write)
				return apiErr
			},
		})
	}
}

func isResponsesImageFunction(tool gjson.Result) bool {
	return tool.Get("type").String() == "function" && (tool.Get("name").String() == "image_gen.imagegen" ||
		tool.Get("namespace").String() == "image_gen" && tool.Get("name").String() == "imagegen")
}

func hasResponsesImageTool(body []byte) bool {
	for _, tool := range gjson.GetBytes(body, "tools").Array() {
		if isResponsesImageFunction(tool) {
			return true
		}
		if tool.Get("type").String() == "namespace" && tool.Get("name").String() == "image_gen" {
			for _, function := range tool.Get("tools").Array() {
				if function.Get("name").String() == "imagegen" && function.Get("type").String() == "function" {
					return true
				}
			}
		}
	}
	return false
}

func prepareResponsesImageTool(body []byte) (*responsesImageTool, error) {
	if gjson.GetBytes(body, "background").Bool() || gjson.GetBytes(body, "conversation").Value() != nil {
		return nil, errors.New("background and conversation are not supported by the internal image tool")
	}
	choice := gjson.GetBytes(body, "tool_choice").Raw
	if choice == "" {
		choice = `"auto"`
	}
	tool := &responsesImageTool{tools: []byte(gjson.GetBytes(body, "tools").Raw), choice: []byte(choice), name: "newapi_image_generate_" + common.GetUUID(), allowed: true}
	tools := make([]common.RawMessage, 0)
	for _, item := range gjson.GetBytes(body, "tools").Array() {
		if item.Get("type").String() == "image_generation" {
			if tool.settings.Exists() {
				return nil, errors.New("only one image_generation tool is supported")
			}
			tool.settings = item
			continue
		}
		if isResponsesImageFunction(item) {
			continue
		}
		if item.Get("type").String() == "namespace" && item.Get("name").String() == "image_gen" {
			functions := make([]common.RawMessage, 0)
			for _, function := range item.Get("tools").Array() {
				if function.Get("name").String() != "imagegen" || function.Get("type").String() != "function" {
					functions = append(functions, []byte(function.Raw))
				}
			}
			if len(functions) != 0 {
				updated, err := sjson.Set(item.Raw, "tools", functions)
				if err != nil {
					return nil, err
				}
				tools = append(tools, []byte(updated))
			}
			continue
		}
		tools = append(tools, []byte(item.Raw))
	}
	if action := tool.settings.Get("action").String(); action != "" && action != "auto" && action != "generate" || tool.settings.Get("input_image_mask").Exists() {
		return nil, errors.New("the internal image tool supports generation, not image editing or masks")
	}
	function, err := common.Marshal(gin.H{
		"type": "function", "name": tool.name, "description": "Generate one new image from a detailed prompt. Do not use for editing existing images.", "strict": true,
		"parameters": gin.H{"type": "object", "properties": gin.H{"prompt": gin.H{"type": "string"}}, "required": []string{"prompt"}, "additionalProperties": false},
	})
	if err != nil {
		return nil, err
	}
	tools = append(tools, function)
	prepared, err := sjson.SetBytes(body, "tools", tools)
	if err != nil {
		return nil, err
	}
	selection := gjson.GetBytes(body, "tool_choice")
	tool.allowed = selection.String() != "none"
	if selection.Get("type").String() == "image_generation" || isResponsesImageFunction(selection) {
		prepared, err = sjson.SetBytes(prepared, "tool_choice", gin.H{"type": "function", "name": tool.name})
	} else if selection.Get("type").String() == "allowed_tools" {
		allowed := make([]common.RawMessage, 0)
		imageAllowed := false
		for _, item := range selection.Get("tools").Array() {
			if item.Get("type").String() == "image_generation" || isResponsesImageFunction(item) {
				imageAllowed = true
				continue
			}
			allowed = append(allowed, []byte(item.Raw))
		}
		if imageAllowed {
			entry, marshalErr := common.Marshal(gin.H{"type": "function", "name": tool.name})
			if marshalErr != nil {
				return nil, marshalErr
			}
			allowed = append(allowed, entry)
		}
		tool.allowed = imageAllowed && selection.Get("mode").String() != "none"
		prepared, err = sjson.SetBytes(prepared, "tool_choice.tools", allowed)
	} else if selection.IsObject() {
		tool.allowed = false
	}
	if err != nil {
		return nil, err
	}
	// 每轮最多生成一张，避免模型并行调用导致意外重复扣费。
	prepared, err = sjson.SetBytes(prepared, "parallel_tool_calls", false)
	if err != nil {
		return nil, err
	}
	tool.request, err = sjson.SetBytes(prepared, "stream", true)
	return tool, err
}

func (tool *responsesImageTool) restoreMetadata(response []byte) ([]byte, error) {
	response, err := sjson.SetRawBytes(response, "tools", tool.tools)
	if err != nil {
		return nil, err
	}
	return sjson.SetRawBytes(response, "tool_choice", tool.choice)
}

func runResponsesImageTool(c *gin.Context, dispatcher http.Handler, body []byte, write helper.ResponsesStreamWriter) ([]byte, *types.NewAPIError) {
	tool, err := prepareResponsesImageTool(body)
	if err != nil {
		return nil, responsesImageToolError(c, http.StatusBadRequest, err)
	}
	first := dispatchResponsesImageTool(c, dispatcher, "/v1/responses", tool.request, nil)
	if apiErr := first.finishResponse(c); apiErr != nil {
		return nil, apiErr
	}
	var call gjson.Result
	otherCalls := false
	for _, item := range gjson.GetBytes(first.response, "output").Array() {
		if item.Get("type").String() == "function_call" && item.Get("name").String() == tool.name {
			if call.Exists() {
				return nil, responsesImageToolError(c, http.StatusBadGateway, errors.New("multiple image calls were returned; no image was submitted"))
			}
			call = item
		} else if item.Get("type").String() == "function_call" || item.Get("type").String() == "custom_tool_call" {
			otherCalls = true
		}
	}
	if call.Exists() && gjson.GetBytes(first.response, "status").String() != "completed" {
		return nil, responsesImageToolError(c, http.StatusBadGateway, errors.New("the image function call was incomplete; no image was submitted"))
	}
	if !call.Exists() {
		response, err := tool.restoreMetadata(first.response)
		if err != nil {
			return nil, responsesImageToolError(c, http.StatusBadGateway, err)
		}
		for _, data := range first.events {
			if write == nil {
				break
			}
			if gjson.GetBytes(data, "response").Exists() {
				var restored []byte
				restored, err = tool.restoreMetadata([]byte(gjson.GetBytes(data, "response").Raw))
				if err == nil {
					data, err = sjson.SetRawBytes(data, "response", restored)
				}
			}
			if err == nil {
				err = write(data)
			}
			if err != nil {
				return nil, responsesImageToolError(c, http.StatusBadGateway, err)
			}
		}
		return response, nil
	}
	if otherCalls || call.Get("call_id").String() == "" {
		return nil, responsesImageToolError(c, http.StatusBadGateway, errors.New("the image tool must be called without other client functions"))
	}
	if !tool.allowed {
		return nil, responsesImageToolError(c, http.StatusBadGateway, errors.New("the response called an image tool excluded by tool_choice; no image was submitted"))
	}
	imageRequest, err := tool.imageRequest(call.Get("arguments").String())
	if err != nil {
		return nil, responsesImageToolError(c, http.StatusBadRequest, err)
	}
	image := dispatchResponsesImageTool(c, dispatcher, "/v1/images/generations", imageRequest, nil)
	if apiErr := image.apiError(c); apiErr != nil {
		return nil, apiErr
	}
	result, format, err := image.imageResult(c.Request.Context())
	if err != nil {
		return nil, responsesImageToolError(c, http.StatusBadGateway, err)
	}
	item := gin.H{"id": "ig_newapi_" + common.GetUUID(), "type": "image_generation_call", "status": "completed", "result": result, "output_format": format}
	if prompt := gjson.GetBytes(image.body.Bytes(), "data.0.revised_prompt").String(); prompt != "" {
		item["revised_prompt"] = prompt
	}
	itemJSON, err := common.Marshal(item)
	if err != nil {
		return nil, responsesImageToolError(c, http.StatusBadGateway, err)
	}
	// 回填函数结果后再完成聊天，保留可继续使用的上游 response ID，不建立额外的会话缓存。
	var input []common.RawMessage
	originalInput := gjson.GetBytes(tool.request, "input")
	if originalInput.Type == gjson.String {
		message, marshalErr := common.Marshal(gin.H{"role": "user", "content": originalInput.String()})
		if marshalErr != nil {
			return nil, responsesImageToolError(c, http.StatusBadRequest, marshalErr)
		}
		input = append(input, message)
	} else {
		for _, value := range originalInput.Array() {
			input = append(input, []byte(value.Raw))
		}
	}
	for _, value := range gjson.GetBytes(first.response, "output").Array() {
		input = append(input, []byte(value.Raw))
	}
	output, _ := common.Marshal(gin.H{"type": "function_call_output", "call_id": call.Get("call_id").String(), "output": "One image was generated successfully and delivered to the user. Do not generate it again."})
	input = append(input, output)
	continuation, err := sjson.SetBytes(tool.request, "input", input)
	if err == nil {
		continuation, err = sjson.SetBytes(continuation, "tool_choice", "none")
	}
	if err != nil {
		return nil, responsesImageToolError(c, http.StatusBadRequest, err)
	}
	stream := &responsesImageToolStream{write: write, tool: tool, image: itemJSON, first: first.response}
	last := dispatchResponsesImageTool(c, dispatcher, "/v1/responses", continuation, stream.observe)
	if apiErr := last.finishResponse(c); apiErr != nil && len(stream.response) == 0 {
		// 图片已由原路由结算，后续聊天失败仍返回图片，不能重试生成或丢弃已付费结果。
		base := stream.current
		if len(base) == 0 {
			base = first.response
			base, _ = sjson.SetBytes(base, "usage", nil)
		}
		base, _ = sjson.SetBytes(base, "status", "failed")
		base, _ = sjson.SetBytes(base, "output", []any{})
		base, _ = sjson.SetBytes(base, "error", apiErr.ToOpenAIError())
		failed, _ := common.Marshal(gin.H{"type": "response.failed", "response": common.RawMessage(base)})
		if err := stream.observe(failed); err != nil {
			return nil, responsesImageToolError(c, http.StatusBadGateway, err)
		}
	}
	if err := stream.emit(stream.terminal); err != nil {
		return nil, responsesImageToolError(c, http.StatusBadGateway, err)
	}
	return stream.response, nil
}

func (tool *responsesImageTool) imageRequest(arguments string) ([]byte, error) {
	var args struct {
		Prompt string `json:"prompt"`
	}
	if err := common.UnmarshalJsonStr(arguments, &args); err != nil || strings.TrimSpace(args.Prompt) == "" {
		return nil, errors.New("the image tool requires a non-empty prompt")
	}
	model := tool.settings.Get("model").String()
	if model == "" {
		common.OptionMapRWMutex.RLock()
		model = strings.TrimSpace(common.OptionMap["global.responses_image_tool_model"])
		common.OptionMapRWMutex.RUnlock()
	}
	if model == "" {
		model = strings.TrimSpace(os.Getenv("RESPONSES_IMAGE_TOOL_MODEL"))
	}
	if model == "" {
		model = "grok-imagine-image-2.0"
	}
	request := gin.H{"model": model, "prompt": args.Prompt, "n": 1}
	if strings.HasPrefix(model, "grok-imagine-") {
		request["aspect_ratio"], request["resolution"], request["quality"], request["response_format"] = "1:1", "1k", "auto", "b64_json"
		for _, name := range []string{"aspect_ratio", "resolution", "quality"} {
			if value := tool.settings.Get(name); value.Exists() {
				request[name] = common.RawMessage(value.Raw)
			}
		}
	} else {
		request["size"], request["quality"], request["output_format"], request["stream"], request["partial_images"] = "1024x1024", "high", "png", true, 1
		for _, name := range []string{"size", "quality", "output_format", "output_compression", "background", "moderation"} {
			if value := tool.settings.Get(name); value.Exists() {
				request[name] = common.RawMessage(value.Raw)
			}
		}
	}
	return common.Marshal(request)
}

// 子请求只继承调用者请求，不继承 Gin 中已选中的聊天渠道、价格或用户缓存。
func dispatchResponsesImageTool(c *gin.Context, dispatcher http.Handler, path string, body []byte, observe helper.ResponsesStreamWriter) *responsesImageToolExchange {
	exchange := &responsesImageToolExchange{header: make(http.Header), observe: observe}
	ctx := context.WithValue(c.Request.Context(), responsesImageToolContextKey{}, responsesImageToolSubrequest{image: path == "/v1/images/generations"})
	request := c.Request.Clone(ctx)
	request.Method, request.URL.Path, request.URL.RawPath, request.URL.RawQuery = http.MethodPost, path, "", ""
	request.RequestURI = path
	request.Header = c.Request.Header.Clone()
	for _, name := range []string{"Connection", "Upgrade", "Sec-WebSocket-Key", "Sec-WebSocket-Version", "Sec-WebSocket-Extensions", "Sec-WebSocket-Protocol", "Content-Length", "Content-Encoding", "X-Forwarded-For", "X-Real-Ip", "Forwarded"} {
		request.Header.Del(name)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	request.RemoteAddr = net.JoinHostPort(common.GetClientIP(c), "0")
	request.Body, request.ContentLength, request.GetBody = io.NopCloser(bytes.NewReader(body)), int64(len(body)), nil
	request.TransferEncoding = nil
	dispatcher.ServeHTTP(exchange, request)
	return exchange
}

type responsesImageToolExchange struct {
	header      http.Header
	status      int
	body        bytes.Buffer
	pending     []byte
	events      [][]byte
	response    []byte
	failure     common.RawMessage
	errorStatus int
	observe     helper.ResponsesStreamWriter
	used        int
	err         error
}

func (exchange *responsesImageToolExchange) Header() http.Header { return exchange.header }
func (exchange *responsesImageToolExchange) Flush()              {}
func (exchange *responsesImageToolExchange) WriteHeader(status int) {
	if exchange.status == 0 {
		exchange.status = status
	}
}

func (exchange *responsesImageToolExchange) Write(data []byte) (int, error) {
	if exchange.err != nil {
		return 0, exchange.err
	}
	exchange.WriteHeader(http.StatusOK)
	if exchange.used+len(data) > helper.DefaultMaxScannerBufferSize {
		exchange.err = errors.New("internal image tool response is too large")
		return 0, exchange.err
	}
	exchange.used += len(data)
	if !strings.HasPrefix(exchange.header.Get("Content-Type"), "text/event-stream") {
		return exchange.body.Write(data)
	}
	exchange.pending = append(exchange.pending, data...)
	for {
		end := bytes.Index(exchange.pending, []byte("\n\n"))
		separator := 2
		if crlf := bytes.Index(exchange.pending, []byte("\r\n\r\n")); crlf >= 0 && (end < 0 || crlf < end) {
			end, separator = crlf, 4
		}
		if end < 0 {
			break
		}
		var lines []string
		for line := range strings.SplitSeq(string(exchange.pending[:end]), "\n") {
			if value, found := strings.CutPrefix(strings.TrimSuffix(line, "\r"), "data:"); found {
				lines = append(lines, strings.TrimPrefix(value, " "))
			}
		}
		exchange.pending = exchange.pending[end+separator:]
		payload := []byte(strings.Join(lines, "\n"))
		if len(payload) == 0 || bytes.Equal(payload, []byte("[DONE]")) {
			continue
		}
		if !gjson.ValidBytes(payload) {
			exchange.err = errors.New("invalid internal image tool stream event")
			return 0, exchange.err
		}
		if response := gjson.GetBytes(payload, "response"); response.IsObject() && isResponsesImageToolTerminal(gjson.GetBytes(payload, "type").String()) {
			exchange.response = []byte(response.Raw)
		}
		if failure := gjson.GetBytes(payload, "error"); failure.IsObject() {
			exchange.failure = []byte(failure.Raw)
			exchange.errorStatus = int(gjson.GetBytes(payload, "status").Int())
			continue
		}
		if gjson.GetBytes(payload, "type").String() == "error" {
			exchange.failure = payload
			exchange.errorStatus = int(gjson.GetBytes(payload, "status").Int())
			continue
		}
		if exchange.observe != nil {
			exchange.err = exchange.observe(payload)
		} else {
			exchange.events = append(exchange.events, payload)
		}
		if exchange.err != nil {
			return 0, exchange.err
		}
	}
	return len(data), nil
}

func (exchange *responsesImageToolExchange) apiError(c *gin.Context) *types.NewAPIError {
	if exchange.err != nil {
		return responsesImageToolError(c, http.StatusBadGateway, exchange.err)
	}
	if len(exchange.response) == 0 && gjson.GetBytes(exchange.body.Bytes(), "object").String() == "response" {
		exchange.response = bytes.Clone(exchange.body.Bytes())
	}
	rawError := gjson.ParseBytes(exchange.failure)
	if value := gjson.GetBytes(exchange.body.Bytes(), "error"); value.IsObject() {
		rawError = value
	}
	if value := gjson.GetBytes(exchange.response, "error"); value.IsObject() {
		rawError = value
	}
	if rawError.Exists() {
		var apiError types.OpenAIError
		if common.UnmarshalJsonStr(rawError.Raw, &apiError) == nil {
			status := exchange.status
			if exchange.errorStatus >= http.StatusBadRequest && exchange.errorStatus <= 599 {
				status = exchange.errorStatus
			}
			if status < http.StatusBadRequest {
				status = http.StatusBadGateway
			}
			return types.WithOpenAIError(apiError, status, types.ErrOptionWithSkipRetry())
		}
	}
	if exchange.status >= http.StatusBadRequest {
		return responsesImageToolError(c, exchange.status, errors.New(http.StatusText(exchange.status)))
	}
	if len(bytes.TrimSpace(exchange.pending)) > 0 {
		return responsesImageToolError(c, http.StatusBadGateway, errors.New("internal image tool stream ended before its final event"))
	}
	return nil
}

// finishResponse 要求完整的 Responses 终态，也将上游 JSON 回退转成同样的事件序列。
func (exchange *responsesImageToolExchange) finishResponse(c *gin.Context) *types.NewAPIError {
	if apiErr := exchange.apiError(c); apiErr != nil {
		return apiErr
	}
	status := gjson.GetBytes(exchange.response, "status").String()
	if gjson.GetBytes(exchange.response, "id").String() == "" || !isResponsesImageToolTerminal("response."+status) {
		return responsesImageToolError(c, http.StatusBadGateway, errors.New("internal chat response ended without a final response"))
	}
	if strings.HasPrefix(exchange.header.Get("Content-Type"), "text/event-stream") {
		return nil
	}
	created, _ := sjson.SetBytes(exchange.response, "output", []any{})
	created, _ = sjson.SetBytes(created, "status", "in_progress")
	created, _ = sjson.SetBytes(created, "usage", nil)
	created, _ = sjson.SetBytes(created, "error", nil)
	events := []gin.H{{"type": "response.created", "response": common.RawMessage(created)}}
	for index, item := range gjson.GetBytes(exchange.response, "output").Array() {
		events = append(events, gin.H{"type": "response.output_item.added", "output_index": index, "item": common.RawMessage(item.Raw)})
		for contentIndex, part := range item.Get("content").Array() {
			if part.Get("type").String() == "output_text" {
				events = append(events, gin.H{"type": "response.output_text.delta", "output_index": index, "content_index": contentIndex, "item_id": item.Get("id").String(), "delta": part.Get("text").String()})
			}
		}
		if item.Get("type").String() == "function_call" {
			events = append(events, gin.H{"type": "response.function_call_arguments.done", "output_index": index, "item_id": item.Get("id").String(), "arguments": item.Get("arguments").String()})
		}
		events = append(events, gin.H{"type": "response.output_item.done", "output_index": index, "item": common.RawMessage(item.Raw)})
	}
	events = append(events, gin.H{"type": "response." + status, "response": common.RawMessage(exchange.response)})
	for sequence, event := range events {
		event["sequence_number"] = sequence
		data, err := common.Marshal(event)
		if err == nil && exchange.observe != nil {
			err = exchange.observe(data)
		} else if err == nil {
			exchange.events = append(exchange.events, data)
		}
		if err != nil {
			return responsesImageToolError(c, http.StatusBadGateway, err)
		}
	}
	return nil
}

func (exchange *responsesImageToolExchange) imageResult(ctx context.Context) (string, string, error) {
	var payload, imageURL string
	for _, event := range exchange.events {
		if gjson.GetBytes(event, "type").String() == "image_generation.completed" {
			payload = gjson.GetBytes(event, "b64_json").String()
		}
	}
	data := gjson.GetBytes(exchange.body.Bytes(), "data")
	entries := data.Array()
	if data.IsObject() {
		entries = []gjson.Result{data}
	}
	for _, entry := range entries {
		if value := entry.Get("b64_json").String(); value != "" {
			payload = value
			break
		}
		if imageURL == "" {
			imageURL = entry.Get("url").String()
		}
	}
	if payload == "" && imageURL != "" {
		if err := service.ValidateSSRFProtectedFetchURL(imageURL); err != nil {
			return "", "", errors.New("the generated image URL was rejected by the fetch policy")
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
		if err != nil {
			return "", "", errors.New("invalid generated image URL")
		}
		// 不复制聊天令牌或渠道凭据到图片下载地址。
		response, err := service.GetSSRFProtectedHTTPClient().Do(request)
		if err != nil {
			return "", "", errors.New("failed to download the generated image")
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return "", "", fmt.Errorf("image download returned HTTP %d", response.StatusCode)
		}
		limit := int64(constant.MaxFileDownloadMB) << 20
		if limit <= 0 {
			limit = 64 << 20
		}
		data, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
		if err != nil || int64(len(data)) > limit {
			return "", "", errors.New("failed to read the generated image within the download limit")
		}
		payload = base64.StdEncoding.EncodeToString(data)
	}
	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil || len(decoded) == 0 || !strings.HasPrefix(http.DetectContentType(decoded), "image/") {
		return "", "", errors.New("image generation returned no complete image payload")
	}
	format := strings.TrimPrefix(http.DetectContentType(decoded), "image/")
	return payload, format, nil
}

type responsesImageToolStream struct {
	write    helper.ResponsesStreamWriter
	tool     *responsesImageTool
	image    common.RawMessage
	first    []byte
	current  []byte
	response []byte
	terminal []byte
	sequence int
	inserted bool
}

func (stream *responsesImageToolStream) emit(event []byte) error {
	if stream.write == nil {
		return nil
	}
	event, err := sjson.SetBytes(event, "sequence_number", stream.sequence)
	if err != nil {
		return err
	}
	stream.sequence++
	return stream.write(event)
}

func (stream *responsesImageToolStream) observe(event []byte) error {
	outputs := gjson.GetBytes(event, "response.output").Array()
	if item := gjson.GetBytes(event, "item"); item.Exists() {
		outputs = append(outputs, item)
	}
	for _, item := range outputs {
		if item.Get("type").String() == "function_call" && item.Get("name").String() == stream.tool.name {
			return errors.New("the continuation tried to generate another image despite tool_choice=none")
		}
	}
	var err error
	if response := gjson.GetBytes(event, "response"); response.IsObject() {
		stream.current, err = stream.tool.restoreMetadata([]byte(response.Raw))
		if err == nil {
			event, err = sjson.SetRawBytes(event, "response", stream.current)
		}
		if err != nil {
			return err
		}
	}
	terminal := isResponsesImageToolTerminal(gjson.GetBytes(event, "type").String())
	if !stream.inserted && (gjson.GetBytes(event, "type").String() == "response.created" || terminal) {
		created := event
		if terminal {
			created, _ = sjson.SetBytes(event, "type", "response.created")
			created, _ = sjson.SetBytes(created, "response.status", "in_progress")
			created, _ = sjson.SetBytes(created, "response.output", []any{})
			created, _ = sjson.SetBytes(created, "response.usage", nil)
			created, _ = sjson.SetBytes(created, "response.error", nil)
		}
		if err = stream.emit(created); err != nil {
			return err
		}
		imageID := gjson.GetBytes(stream.image, "id").String()
		started, _ := common.Marshal(gin.H{"type": "response.output_item.added", "output_index": 0, "item": gin.H{"id": imageID, "type": "image_generation_call", "status": "in_progress"}})
		if err = stream.emit(started); err != nil {
			return err
		}
		done, _ := common.Marshal(gin.H{"type": "response.output_item.done", "output_index": 0, "item": stream.image})
		stream.inserted = true
		if err = stream.emit(done); err != nil || !terminal {
			return err
		}
	}
	if index := gjson.GetBytes(event, "output_index"); index.Exists() {
		event, err = sjson.SetBytes(event, "output_index", index.Int()+1)
		if err != nil {
			return err
		}
	}
	if terminal {
		outputs := []common.RawMessage{stream.image}
		for _, output := range gjson.GetBytes(event, "response.output").Array() {
			outputs = append(outputs, []byte(output.Raw))
		}
		event, err = sjson.SetBytes(event, "response.output", outputs)
		if err != nil {
			return err
		}
		for _, key := range []string{"input_tokens", "output_tokens", "total_tokens", "input_tokens_details.cached_tokens", "output_tokens_details.reasoning_tokens"} {
			prior := gjson.GetBytes(stream.first, "usage."+key)
			current := gjson.GetBytes(event, "response.usage."+key)
			if prior.Exists() || current.Exists() {
				// 这里只合并客户端展示用量，扣费由各子请求原有流程独立结算。
				value := min(max(prior.Int(), 0), int64(common.MaxQuota)) + min(max(current.Int(), 0), int64(common.MaxQuota))
				event, err = sjson.SetBytes(event, "response.usage."+key, value)
				if err != nil {
					return err
				}
			}
		}
		stream.response = []byte(gjson.GetBytes(event, "response").Raw)
		stream.terminal = event
		return nil
	}
	return stream.emit(event)
}

func isResponsesImageToolTerminal(event string) bool {
	switch event {
	case "response.completed", "response.done", "response.incomplete", "response.failed", "response.cancelled", "response.canceled":
		return true
	}
	return false
}

func responsesImageToolError(c *gin.Context, status int, err error) *types.NewAPIError {
	typeName := "api_error"
	if status == http.StatusBadRequest {
		typeName = "invalid_request_error"
	}
	return types.WithOpenAIError(types.OpenAIError{Type: typeName, Message: i18n.T(c, i18n.MsgRelayImageToolFailed, map[string]any{"Error": err.Error()}), Code: "image_tool_error"}, status, types.ErrOptionWithSkipRetry())
}

func writeResponsesImageToolError(write helper.ResponsesStreamWriter, apiErr *types.NewAPIError) error {
	if write == nil {
		return nil
	}
	payload, err := common.Marshal(gin.H{"type": "error", "status": apiErr.StatusCode, "error": apiErr.ToOpenAIError()})
	if err != nil {
		return err
	}
	return write(payload)
}

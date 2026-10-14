package relay

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/tidwall/gjson"
)

// 每条连接只保留一个尚未生成响应的追加指令，预留额度由其独立请求管理。
type responsesWSSteering struct {
	frame        []byte
	parentID     string
	ready        chan struct{}
	readyOnce    sync.Once
	active       chan struct{}
	stop         chan struct{}
	stopped      bool
	waitingTools bool
	started      bool
}

func (s *responsesWSSession) nextRequestID() string {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	id := fmt.Sprintf("%s-ws-%d", s.requestID, s.nextEventIndex)
	s.nextEventIndex++
	return id
}

// 派生的 HTTP 形状请求仅用于服务端鉴权、预留和结算，发送上游的仍是原始 steer 帧。
func buildResponsesWSSteeringCreate(parent responsesWSCreateRequest, prepared, frame []byte, responseID string) ([]byte, responsesWSCreateEvent, error) {
	var fields map[string]common.RawMessage
	if err := common.Unmarshal(frame, &fields); err != nil {
		return nil, responsesWSCreateEvent{}, err
	}
	for key := range fields {
		switch key {
		case "type", "previous_response_id", "input", "event_id", "stream_id":
		default:
			return nil, responsesWSCreateEvent{}, fmt.Errorf("response.steer cannot override %s", key)
		}
	}
	if gjson.GetBytes(frame, "previous_response_id").String() != responseID {
		return nil, responsesWSCreateEvent{}, errors.New("response.steer must target the active response")
	}
	input := gjson.GetBytes(frame, "input")
	valid := input.Type == gjson.String && strings.TrimSpace(input.String()) != ""
	if input.IsArray() && len(input.Array()) > 0 {
		valid = true
		for _, message := range input.Array() {
			if !message.IsObject() || message.Get("role").String() != "user" || (message.Get("type").Exists() && message.Get("type").String() != "message") {
				valid = false
				break
			}
			content := message.Get("content")
			if content.Type == gjson.String && strings.TrimSpace(content.String()) != "" {
				continue
			}
			if !content.IsArray() || len(content.Array()) == 0 {
				valid = false
				break
			}
			for _, part := range content.Array() {
				switch part.Get("type").String() {
				case "input_text", "input_image", "input_file":
				default:
					valid = false
				}
			}
		}
	}
	if !valid {
		return nil, responsesWSCreateEvent{}, errors.New("response.steer input must be nonempty text or user messages containing input_text, input_image, or input_file")
	}
	var inherited map[string]common.RawMessage
	if err := common.Unmarshal(prepared, &inherited); err != nil {
		return nil, responsesWSCreateEvent{}, err
	}
	inherited["input"] = fields["input"]
	inherited["previous_response_id"] = fields["previous_response_id"]
	inherited["model"], _ = common.Marshal(parent.Request.Model)
	delete(inherited, "generate")
	delete(inherited, "event_id")
	if eventID := fields["event_id"]; len(eventID) > 0 {
		inherited["event_id"] = eventID
	}
	message, err := common.Marshal(inherited)
	if err != nil {
		return nil, responsesWSCreateEvent{}, err
	}
	envelope, streamID, err := parseResponsesWSEnvelope(message)
	if err == nil && streamID != parent.StreamID {
		err = errors.New("response.steer must use the active stream")
	}
	if requested := gjson.GetBytes(frame, "stream_id"); requested.Exists() && requested.String() != parent.StreamID {
		err = errors.New("response.steer must use the active stream")
	}
	return message, envelope, err
}

func (s *responsesWSSession) queueSteering(parentState *responsesWSCallState, parent responsesWSCreateRequest, responseID string, control responsesWSControl) error {
	// 参数转换会改写 DTO 的模型名；重新鉴权必须使用连接绑定的客户端模型。
	parent.Request.Model = s.lockedModel
	message, envelope, err := buildResponsesWSSteeringCreate(parent, parentState.preparedBody, control.body, responseID)
	if err != nil {
		return err
	}
	steer := &responsesWSSteering{frame: bytes.Clone(control.body), parentID: responseID, ready: make(chan struct{}), active: make(chan struct{}), stop: make(chan struct{})}
	state := &responsesWSCallState{inbox: make(chan responsesWSMessage), controls: make(chan responsesWSControl, 1), done: make(chan struct{}), steer: steer, preparedBody: bytes.Clone(parentState.preparedBody)}
	s.stateMu.Lock()
	if s.pendingSteer != nil || parentState.steer != nil && !parentState.steer.started {
		s.stateMu.Unlock()
		return errors.New("a response.steer is already pending")
	}
	s.pendingSteer = state
	s.stateMu.Unlock()
	requestID := s.nextRequestID()
	s.workers.Go(func() { s.runRequest(state, message, envelope, parent.StreamID, requestID) })
	select {
	case <-steer.ready:
	case <-s.ctx.Done():
	}
	return nil
}

func (s *responsesWSSession) handleSteeringEvent(state *responsesWSCallState, incoming responsesWSMessage) {
	s.stateMu.Lock()
	pending := s.pendingSteer
	if pending == nil && state.steer != nil && !state.steer.started {
		pending = state
	}
	if pending != nil && gjson.GetBytes(incoming.body, "steer.previous_response_id").String() == pending.steer.parentID {
		switch gjson.GetBytes(incoming.body, "type").String() {
		case "response.steer.failed":
			if !pending.steer.stopped {
				pending.steer.stopped = true
				close(pending.steer.stop)
			}
			if s.pendingSteer == pending {
				s.pendingSteer = nil
			}
		case "response.steer.pending":
			pending.steer.waitingTools = true
		}
	}
	s.stateMu.Unlock()
	if err := s.writeClient(incoming.kind, incoming.body); err != nil {
		s.shutdown()
	}
}

// 顶层 error 只有可关联到追加指令时才取消该预留，不结束仍在生成的父响应。
func (s *responsesWSSession) rejectSteeringControl(event responsesWSErrorEvent) bool {
	if event.Status != 0 && event.Status != 400 || event.Error == nil || event.Error.Type != "invalid_request_error" {
		return false
	}
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	pending := s.pendingSteer
	if pending == nil {
		return false
	}
	id := gjson.GetBytes(pending.steer.frame, "event_id").String()
	code := ""
	if event.Error != nil && event.Error.Code != nil {
		code = fmt.Sprint(event.Error.Code)
	}
	if !(id != "" && id == event.EventID || strings.HasPrefix(code, "steer") && (event.ResponseID == "" || event.ResponseID == pending.steer.parentID)) {
		return false
	}
	if !pending.steer.stopped {
		pending.steer.stopped = true
		close(pending.steer.stop)
	}
	s.pendingSteer = nil
	return true
}

// 等待工具结果时，显式 create 使用自己的配置和计费；尚未生成的隐式预留先退还。
func (s *responsesWSSession) releaseSteeringForContinuation(message []byte, envelope responsesWSCreateEvent, streamID string) bool {
	s.stateMu.Lock()
	state := s.current
	if state == nil || state.steer == nil || state.steer.started || !state.steer.waitingTools {
		s.stateMu.Unlock()
		return true
	}
	create, err := normalizeResponsesWSCreateEvent(message, envelope, streamID)
	if err != nil || create.Request.PreviousResponseID != state.steer.parentID {
		s.stateMu.Unlock()
		s.sendError(envelope.EventID, streamID, newResponsesWSInvalidRequestError(errors.New("tool continuation must reference the response targeted by the pending steer")))
		return false
	}
	if !state.steer.stopped {
		state.steer.stopped = true
		close(state.steer.stop)
	}
	s.stateMu.Unlock()
	select {
	case <-state.done:
		return true
	case <-s.ctx.Done():
		return false
	}
}

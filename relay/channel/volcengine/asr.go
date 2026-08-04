package volcengine

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const (
	volcengineASRWebSocketURL       = "wss://openspeech.bytedance.com/api/v3/sauc/bigmodel_nostream"
	volcengineArkSoutheastBaseURL   = "https://ark.ap-southeast.bytepluses.com"
	volcengineBigASRResourceID      = "volc.bigasr.sauc.duration"
	volcengineSeedASRResourceID     = "volc.seedasr.sauc.duration"
	volcengineASRDefaultChunkSize   = 8 * 1024
	volcengineASRMaxChunkSize       = 256 * 1024
	volcengineASRWebSocketTimeout   = 10 * time.Minute
	volcengineASRDefaultSampleRate  = 16000
	volcengineASRDefaultSampleBits  = 16
	volcengineASRDefaultChannelNums = 1
)

type VolcengineASRSession struct {
	Request        VolcengineASRRequest
	AudioData      []byte
	ResponseFormat string
	ResourceID     string
	RequestID      string
	ChunkSize      int
}

type VolcengineASRRequest struct {
	User    map[string]any `json:"user,omitempty"`
	Audio   map[string]any `json:"audio"`
	Request map[string]any `json:"request"`
}

type VolcengineASRMetadata struct {
	ResourceID string         `json:"resource_id,omitempty"`
	Endpoint   string         `json:"endpoint,omitempty"`
	ChunkSize  *int           `json:"chunk_size,omitempty"`
	User       map[string]any `json:"user,omitempty"`
	Audio      map[string]any `json:"audio,omitempty"`
	Request    map[string]any `json:"request,omitempty"`
}

type VolcengineASRResponsePayload struct {
	AudioInfo *VolcengineASRAudioInfo `json:"audio_info,omitempty"`
	Result    json.RawMessage         `json:"result,omitempty"`
}

type VolcengineASRAudioInfo struct {
	Duration float64 `json:"duration,omitempty"`
}

type VolcengineASRResult struct {
	Text       string                     `json:"text,omitempty"`
	Utterances []VolcengineASRUtterance   `json:"utterances,omitempty"`
	Additions  map[string]json.RawMessage `json:"additions,omitempty"`
}

type VolcengineASRUtterance struct {
	Text      string `json:"text,omitempty"`
	StartTime int    `json:"start_time,omitempty"`
	EndTime   int    `json:"end_time,omitempty"`
	Definite  bool   `json:"definite,omitempty"`
}

type VolcengineASROpenAIResponse struct {
	Text       string                   `json:"text"`
	Duration   float64                  `json:"duration,omitempty"`
	Utterances []VolcengineASRUtterance `json:"utterances,omitempty"`
}

func buildVolcengineASRSession(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (VolcengineASRSession, error) {
	formData, err := common.ParseMultipartFormReusable(c)
	if err != nil {
		return VolcengineASRSession{}, fmt.Errorf("error parsing multipart form: %w", err)
	}

	fileHeaders := formData.File["file"]
	if len(fileHeaders) == 0 {
		return VolcengineASRSession{}, errors.New("file is required")
	}
	fileHeader := fileHeaders[0]
	file, err := fileHeader.Open()
	if err != nil {
		return VolcengineASRSession{}, fmt.Errorf("error opening audio file: %w", err)
	}
	defer file.Close()

	audioData, err := io.ReadAll(file)
	if err != nil {
		return VolcengineASRSession{}, fmt.Errorf("read audio file failed: %w", err)
	}
	if len(audioData) == 0 {
		return VolcengineASRSession{}, errors.New("file is empty")
	}

	audioFormat, codec, err := resolveVolcengineASRAudioFormat(formFirstValue(formData.Value, "audio_format", "format"), fileHeader.Filename, formFirstValue(formData.Value, "codec"))
	if err != nil {
		return VolcengineASRSession{}, err
	}

	responseFormat := strings.TrimSpace(formFirstValue(formData.Value, "response_format"))
	if responseFormat == "" {
		responseFormat = request.ResponseFormat
	}

	modelName := request.Model
	if info != nil && strings.TrimSpace(info.OriginModelName) != "" {
		modelName = info.OriginModelName
	}

	resourceID := strings.TrimSpace(formFirstValue(formData.Value, "resource_id", "x_api_resource_id", "X-Api-Resource-Id"))
	if resourceID == "" {
		configuredURL := ""
		if info != nil {
			configuredURL = info.ChannelBaseUrl
		}
		resourceID = defaultVolcengineASRResourceID(modelName, configuredURL)
	}

	requestID := strings.TrimSpace(formFirstValue(formData.Value, "request_id", "X-Api-Request-Id", "x_api_request_id"))
	if requestID == "" && info != nil {
		requestID = strings.TrimSpace(info.RequestId)
	}
	if requestID == "" {
		requestID = generateRequestID()
	}

	language := strings.TrimSpace(formFirstValue(formData.Value, "language"))
	audio := map[string]any{
		"format":  audioFormat,
		"codec":   codec,
		"rate":    parseVolcengineASRInt(formData.Value, volcengineASRDefaultSampleRate, "rate", "sample_rate"),
		"bits":    parseVolcengineASRInt(formData.Value, volcengineASRDefaultSampleBits, "bits"),
		"channel": parseVolcengineASRInt(formData.Value, volcengineASRDefaultChannelNums, "channel", "channels"),
	}
	if language != "" {
		audio["language"] = language
	}

	requestFields := map[string]any{
		"model_name":  "bigmodel",
		"enable_itn":  true,
		"enable_punc": true,
	}
	if responseFormat == "verbose_json" {
		requestFields["show_utterances"] = true
	}
	applyVolcengineASRBoolFields(formData.Value, requestFields,
		"enable_nonstream",
		"enable_itn",
		"enable_speaker_info",
		"enable_punc",
		"enable_ddc",
		"enable_auto_lang",
		"show_utterances",
		"show_speech_rate",
		"show_volume",
		"enable_lid",
		"enable_emotion_detection",
		"enable_gender_detection",
		"enable_accelerate_text",
		"enable_poi_fc",
		"enable_music_fc",
	)
	applyVolcengineASRIntFields(formData.Value, requestFields,
		"accelerate_score",
		"vad_segment_duration",
		"end_window_size",
		"force_to_speech_time",
	)
	applyVolcengineASRStringFields(formData.Value, requestFields,
		"ssd_version",
		"output_zh_variant",
		"result_type",
		"sensitive_words_filter",
	)

	session := VolcengineASRSession{
		Request: VolcengineASRRequest{
			User: map[string]any{
				"uid": "openai_relay_user",
			},
			Audio:   audio,
			Request: requestFields,
		},
		AudioData:      audioData,
		ResponseFormat: responseFormat,
		ResourceID:     resourceID,
		RequestID:      requestID,
		ChunkSize:      volcengineASRDefaultChunkSize,
	}

	if err := applyVolcengineASRMetadata(&session, formFirstValue(formData.Value, "metadata")); err != nil {
		return VolcengineASRSession{}, err
	}
	if chunkSize := parseVolcengineASRInt(formData.Value, 0, "chunk_size"); chunkSize > 0 {
		if err := setVolcengineASRChunkSize(&session, chunkSize); err != nil {
			return VolcengineASRSession{}, err
		}
	}

	return session, nil
}

func handleASRWebSocketResponse(c *gin.Context, requestURL string, session VolcengineASRSession, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	header, headerErr := buildVolcengineASRHeaders(info.ApiKey, session.ResourceID, session.RequestID)
	if headerErr != nil {
		return nil, types.NewErrorWithStatusCode(
			headerErr,
			types.ErrorCodeChannelInvalidKey,
			http.StatusUnauthorized,
		)
	}

	dialer, dialerErr := service.NewWebsocketDialerWithProxy(info.ChannelSetting.Proxy)
	if dialerErr != nil {
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("new websocket dialer failed: %w", dialerErr),
			types.ErrorCodeBadResponseStatusCode,
			http.StatusBadGateway,
		)
	}

	ctx := context.Background()
	if c != nil && c.Request != nil {
		ctx = c.Request.Context()
	}
	conn, resp, dialErr := dialer.DialContext(ctx, requestURL, header)
	if dialErr != nil {
		if resp != nil {
			return nil, types.NewErrorWithStatusCode(
				fmt.Errorf("failed to connect to volcengine ASR websocket: %w, status: %d", dialErr, resp.StatusCode),
				types.ErrorCodeBadResponseStatusCode,
				http.StatusBadGateway,
			)
		}
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("failed to connect to volcengine ASR websocket: %w", dialErr),
			types.ErrorCodeBadResponseStatusCode,
			http.StatusBadGateway,
		)
	}
	defer conn.Close()

	if resp != nil {
		if logID := strings.TrimSpace(resp.Header.Get("X-Tt-Logid")); logID != "" {
			c.Header("X-Tt-Logid", logID)
		}
		if connectID := strings.TrimSpace(resp.Header.Get("X-Api-Connect-Id")); connectID != "" {
			c.Header("X-Api-Connect-Id", connectID)
		}
	}

	if deadlineErr := conn.SetReadDeadline(time.Now().Add(volcengineASRWebSocketTimeout)); deadlineErr != nil {
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("set websocket read deadline failed: %w", deadlineErr),
			types.ErrorCodeBadResponse,
			http.StatusInternalServerError,
		)
	}

	payload, marshalErr := common.Marshal(session.Request)
	if marshalErr != nil {
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("failed to marshal volcengine ASR request: %w", marshalErr),
			types.ErrorCodeBadRequestBody,
			http.StatusInternalServerError,
		)
	}
	agentPlanProtocol := isVolcengineAgentPlanASRURL(requestURL)
	if sendErr := sendVolcengineASRFullClientRequest(conn, payload, agentPlanProtocol); sendErr != nil {
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("failed to send volcengine ASR request: %w", sendErr),
			types.ErrorCodeBadRequestBody,
			http.StatusInternalServerError,
		)
	}
	if sendErr := sendVolcengineASRAudio(conn, session.AudioData, session.ChunkSize, agentPlanProtocol); sendErr != nil {
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("failed to send volcengine ASR audio: %w", sendErr),
			types.ErrorCodeBadRequestBody,
			http.StatusInternalServerError,
		)
	}

	result, receiveErr := receiveVolcengineASRResult(conn)
	if receiveErr != nil {
		return nil, types.NewErrorWithStatusCode(
			receiveErr,
			types.ErrorCodeBadResponse,
			http.StatusBadGateway,
		)
	}

	writeVolcengineASRResponse(c, session.ResponseFormat, result)
	return volcengineASRUsage(info, result.Duration), nil
}

func buildVolcengineASRHeaders(apiKey, resourceID, requestID string) (http.Header, error) {
	apiKey = strings.TrimSpace(apiKey)
	resourceID = strings.TrimSpace(resourceID)
	requestID = strings.TrimSpace(requestID)
	if apiKey == "" {
		return nil, errors.New("api key is required")
	}
	if resourceID == "" {
		return nil, errors.New("resource_id is required")
	}
	if requestID == "" {
		requestID = generateRequestID()
	}

	header := http.Header{}
	parts := strings.Split(apiKey, "|")
	if len(parts) >= 2 && len(parts) <= 3 && strings.TrimSpace(parts[0]) != "" && strings.TrimSpace(parts[1]) != "" {
		header.Set("X-Api-App-Key", strings.TrimSpace(parts[0]))
		header.Set("X-Api-Access-Key", strings.TrimSpace(parts[1]))
	} else {
		header.Set("X-Api-Key", apiKey)
	}
	header.Set("X-Api-Resource-Id", resourceID)
	header.Set("X-Api-Request-Id", requestID)
	header.Set("X-Api-Connect-Id", requestID)
	header.Set("X-Api-Sequence", "-1")
	return header, nil
}

func sendVolcengineASRFullClientRequest(conn *websocket.Conn, payload []byte, agentPlanProtocol bool) error {
	if !agentPlanProtocol {
		return FullClientRequest(conn, payload)
	}
	compressedPayload, err := compressVolcengineASRPayload(payload)
	if err != nil {
		return err
	}
	msg, err := NewMessage(MsgTypeFullClientRequest, MsgTypeFlagPositiveSeq)
	if err != nil {
		return err
	}
	msg.Compression = CompressionGzip
	msg.Sequence = 1
	msg.Payload = compressedPayload
	frame, err := msg.Marshal()
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.BinaryMessage, frame)
}

func sendVolcengineASRAudio(conn *websocket.Conn, audioData []byte, chunkSize int, agentPlanProtocol bool) error {
	if len(audioData) == 0 {
		return errors.New("audio data is empty")
	}
	if chunkSize <= 0 {
		chunkSize = volcengineASRDefaultChunkSize
	}
	sequence := int32(2)
	for offset := 0; offset < len(audioData); offset += chunkSize {
		end := offset + chunkSize
		if end > len(audioData) {
			end = len(audioData)
		}
		isLast := end == len(audioData)
		if !agentPlanProtocol {
			if err := AudioOnlyRequest(conn, audioData[offset:end], sequence, isLast); err != nil {
				return err
			}
		} else if err := sendVolcengineAgentPlanASRAudio(conn, audioData[offset:end], sequence, isLast); err != nil {
			return err
		}
		if sequence == math.MaxInt32 {
			return errors.New("audio sequence overflow")
		}
		sequence++
	}
	return nil
}

func sendVolcengineAgentPlanASRAudio(conn *websocket.Conn, payload []byte, sequence int32, isLast bool) error {
	compressedPayload, err := compressVolcengineASRPayload(payload)
	if err != nil {
		return err
	}
	flag := MsgTypeFlagPositiveSeq
	if isLast {
		flag = MsgTypeFlagNegativeSeq
		if sequence > 0 {
			sequence = -sequence
		}
	}
	msg, err := NewMessage(MsgTypeAudioOnlyClient, flag)
	if err != nil {
		return err
	}
	msg.Serialization = SerializationNone
	msg.Compression = CompressionGzip
	msg.Sequence = sequence
	msg.Payload = compressedPayload
	frame, err := msg.Marshal()
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.BinaryMessage, frame)
}

func compressVolcengineASRPayload(payload []byte) ([]byte, error) {
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write(payload); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return compressed.Bytes(), nil
}

func isVolcengineAgentPlanASRURL(requestURL string) bool {
	normalizedURL := strings.ToLower(strings.TrimSpace(requestURL))
	isWebSocket := strings.HasPrefix(normalizedURL, "ws://") || strings.HasPrefix(normalizedURL, "wss://")
	return isWebSocket && strings.Contains(normalizedURL, "/api/v3/plan/sauc/")
}

func receiveVolcengineASRResult(conn *websocket.Conn) (VolcengineASROpenAIResponse, error) {
	var latest VolcengineASROpenAIResponse
	hasResult := false

	for {
		msg, err := ReceiveMessage(conn)
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				if hasResult {
					return latest, nil
				}
			}
			return VolcengineASROpenAIResponse{}, fmt.Errorf("failed to receive volcengine ASR message: %w", err)
		}
		payload, err := decodeVolcengineASRPayload(msg)
		if err != nil {
			return VolcengineASROpenAIResponse{}, fmt.Errorf("failed to decode volcengine ASR message: %w", err)
		}

		switch msg.MsgType {
		case MsgTypeError:
			return VolcengineASROpenAIResponse{}, fmt.Errorf("received error from volcengine ASR: code=%d, %s", msg.ErrorCode, string(payload))
		case MsgTypeFullServerResponse:
			result, ok, parseErr := parseVolcengineASRResponsePayload(payload)
			if parseErr != nil {
				return VolcengineASROpenAIResponse{}, parseErr
			}
			if ok {
				latest = result
				hasResult = true
			}
			if msg.MsgTypeFlag == MsgTypeFlagNegativeSeq || msg.Sequence < 0 {
				if hasResult {
					return latest, nil
				}
				return VolcengineASROpenAIResponse{}, errors.New("volcengine ASR final response does not include result")
			}
		default:
			continue
		}
	}
}

func decodeVolcengineASRPayload(msg *Message) ([]byte, error) {
	if msg == nil {
		return nil, errors.New("message is nil")
	}
	if msg.Compression != CompressionGzip {
		return msg.Payload, nil
	}
	reader, err := gzip.NewReader(bytes.NewReader(msg.Payload))
	if err != nil {
		return nil, err
	}
	decompressed, err := io.ReadAll(reader)
	if err != nil {
		_ = reader.Close()
		return nil, err
	}
	if err := reader.Close(); err != nil {
		return nil, err
	}
	return decompressed, nil
}

func parseVolcengineASRResponsePayload(payload []byte) (VolcengineASROpenAIResponse, bool, error) {
	if len(payload) == 0 {
		return VolcengineASROpenAIResponse{}, false, nil
	}
	var responsePayload VolcengineASRResponsePayload
	if err := common.Unmarshal(payload, &responsePayload); err != nil {
		return VolcengineASROpenAIResponse{}, false, fmt.Errorf("failed to parse volcengine ASR response: %w", err)
	}

	result, hasResult, err := parseVolcengineASRResult(responsePayload.Result)
	if err != nil {
		return VolcengineASROpenAIResponse{}, false, err
	}

	durationSeconds := 0.0
	if responsePayload.AudioInfo != nil && responsePayload.AudioInfo.Duration > 0 {
		durationSeconds = responsePayload.AudioInfo.Duration / 1000.0
	}
	if !hasResult && durationSeconds <= 0 {
		return VolcengineASROpenAIResponse{}, false, nil
	}

	return VolcengineASROpenAIResponse{
		Text:       result.Text,
		Duration:   durationSeconds,
		Utterances: result.Utterances,
	}, true, nil
}

func parseVolcengineASRResult(raw json.RawMessage) (VolcengineASRResult, bool, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return VolcengineASRResult{}, false, nil
	}

	var single VolcengineASRResult
	if err := common.Unmarshal(raw, &single); err == nil {
		return single, single.Text != "" || len(single.Utterances) > 0, nil
	}

	var list []VolcengineASRResult
	if err := common.Unmarshal(raw, &list); err != nil {
		return VolcengineASRResult{}, false, fmt.Errorf("failed to parse volcengine ASR result: %w", err)
	}
	if len(list) == 0 {
		return VolcengineASRResult{}, false, nil
	}
	merged := VolcengineASRResult{}
	for _, item := range list {
		merged.Text += item.Text
		merged.Utterances = append(merged.Utterances, item.Utterances...)
	}
	return merged, merged.Text != "" || len(merged.Utterances) > 0, nil
}

func writeVolcengineASRResponse(c *gin.Context, responseFormat string, result VolcengineASROpenAIResponse) {
	if responseFormat == "text" {
		c.String(http.StatusOK, result.Text)
		return
	}
	c.JSON(http.StatusOK, result)
}

func volcengineASRUsage(info *relaycommon.RelayInfo, durationSeconds float64) *dto.Usage {
	if durationSeconds > 0 {
		tokens, clamp := common.QuotaRoundChecked(math.Ceil(durationSeconds) / 60.0 * 1000)
		if clamp != nil && info != nil && info.QuotaClamp == nil {
			info.QuotaClamp = clamp
		}
		usage := &dto.Usage{
			PromptTokens: tokens,
			TotalTokens:  tokens,
		}
		usage.PromptTokensDetails.AudioTokens = tokens
		return usage
	}

	tokens := 0
	if info != nil {
		tokens = info.GetEstimatePromptTokens()
	}
	usage := &dto.Usage{
		PromptTokens: tokens,
		TotalTokens:  tokens,
	}
	usage.PromptTokensDetails.AudioTokens = tokens
	return usage
}

func resolveVolcengineASRAudioFormat(explicitFormat, filename, explicitCodec string) (string, string, error) {
	audioFormat := strings.ToLower(strings.TrimSpace(explicitFormat))
	if audioFormat == "" {
		audioFormat = strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), ".")
	}
	switch audioFormat {
	case "mpeg":
		audioFormat = "mp3"
	case "oga":
		audioFormat = "ogg"
	case "opus":
		audioFormat = "ogg"
	}

	switch audioFormat {
	case "pcm", "wav", "ogg", "mp3":
	default:
		return "", "", fmt.Errorf("unsupported volcengine ASR audio format %q, supported: pcm, wav, ogg, mp3", audioFormat)
	}

	codec := strings.ToLower(strings.TrimSpace(explicitCodec))
	if codec == "" {
		codec = "raw"
	}
	if audioFormat == "ogg" && explicitCodec == "" {
		codec = "opus"
	}
	return audioFormat, codec, nil
}

func defaultVolcengineASRResourceID(model, configuredURL string) string {
	if isVolcengineAgentPlanASRURL(configuredURL) {
		return volcengineSeedASRResourceID
	}
	name := strings.ToLower(strings.TrimSpace(model))
	if strings.Contains(name, "seed") || strings.Contains(name, "2") {
		return volcengineSeedASRResourceID
	}
	return volcengineBigASRResourceID
}

func applyVolcengineASRMetadata(session *VolcengineASRSession, raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	var metadata VolcengineASRMetadata
	if err := common.UnmarshalJsonStr(raw, &metadata); err != nil {
		return fmt.Errorf("error unmarshalling metadata to volcengine ASR request: %w", err)
	}
	if strings.TrimSpace(metadata.ResourceID) != "" {
		session.ResourceID = strings.TrimSpace(metadata.ResourceID)
	}
	if metadata.User != nil {
		session.Request.User = metadata.User
	}
	if metadata.Audio != nil {
		mergeVolcengineASRMap(session.Request.Audio, metadata.Audio)
	}
	if metadata.Request != nil {
		mergeVolcengineASRMap(session.Request.Request, metadata.Request)
	}
	if metadata.ChunkSize != nil {
		if err := setVolcengineASRChunkSize(session, *metadata.ChunkSize); err != nil {
			return err
		}
	}
	return nil
}

func setVolcengineASRChunkSize(session *VolcengineASRSession, chunkSize int) error {
	if chunkSize <= 0 || chunkSize > volcengineASRMaxChunkSize {
		return fmt.Errorf("chunk_size must be between 1 and %d", volcengineASRMaxChunkSize)
	}
	session.ChunkSize = chunkSize
	return nil
}

func mergeVolcengineASRMap(dst map[string]any, src map[string]any) {
	for key, value := range src {
		dst[key] = value
	}
}

func applyVolcengineASRBoolFields(values map[string][]string, request map[string]any, fields ...string) {
	for _, field := range fields {
		raw := strings.TrimSpace(formFirstValue(values, field))
		if raw == "" {
			continue
		}
		if parsed, err := strconv.ParseBool(raw); err == nil {
			request[field] = parsed
		}
	}
}

func applyVolcengineASRIntFields(values map[string][]string, request map[string]any, fields ...string) {
	for _, field := range fields {
		raw := strings.TrimSpace(formFirstValue(values, field))
		if raw == "" {
			continue
		}
		if parsed, err := strconv.Atoi(raw); err == nil {
			request[field] = parsed
		}
	}
}

func applyVolcengineASRStringFields(values map[string][]string, request map[string]any, fields ...string) {
	for _, field := range fields {
		raw := strings.TrimSpace(formFirstValue(values, field))
		if raw != "" {
			request[field] = raw
		}
	}
}

func parseVolcengineASRInt(values map[string][]string, defaultValue int, fields ...string) int {
	raw := strings.TrimSpace(formFirstValue(values, fields...))
	if raw == "" {
		return defaultValue
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed <= 0 {
		return defaultValue
	}
	return parsed
}

func formFirstValue(values map[string][]string, fields ...string) string {
	for _, field := range fields {
		for _, value := range values[field] {
			if strings.TrimSpace(value) != "" {
				return value
			}
		}
	}
	return ""
}

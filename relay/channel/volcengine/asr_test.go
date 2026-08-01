package volcengine

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	channelconstant "github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetRequestURLForNativeVolcengineASR(t *testing.T) {
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeAudioTranscription,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: channelconstant.ChannelBaseURLs[channelconstant.ChannelTypeVolcEngine],
		},
	}

	requestURL, err := (&Adaptor{}).GetRequestURL(info)

	require.NoError(t, err)
	assert.Equal(t, volcengineASRWebSocketURL, requestURL)
}

func TestGetRequestURLForCustomVolcengineASR(t *testing.T) {
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeAudioTranscription,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://example.com",
		},
	}

	requestURL, err := (&Adaptor{}).GetRequestURL(info)

	require.NoError(t, err)
	assert.Equal(t, "https://example.com/v1/audio/transcriptions", requestURL)
}

func TestBuildVolcengineASRHeadersWithNewAPIKey(t *testing.T) {
	headers, err := buildVolcengineASRHeaders("test-api-key", volcengineBigASRResourceID, "req-1")

	require.NoError(t, err)
	assert.Equal(t, "test-api-key", headers.Get("X-Api-Key"))
	assert.Equal(t, volcengineBigASRResourceID, headers.Get("X-Api-Resource-Id"))
	assert.Equal(t, "req-1", headers.Get("X-Api-Request-Id"))
	assert.Equal(t, "-1", headers.Get("X-Api-Sequence"))
	assert.Empty(t, headers.Get("X-Api-App-Key"))
}

func TestBuildVolcengineASRHeadersWithLegacyKeyPair(t *testing.T) {
	headers, err := buildVolcengineASRHeaders("app-key|access-key", volcengineSeedASRResourceID, "req-2")

	require.NoError(t, err)
	assert.Equal(t, "app-key", headers.Get("X-Api-App-Key"))
	assert.Equal(t, "access-key", headers.Get("X-Api-Access-Key"))
	assert.Empty(t, headers.Get("X-Api-Key"))
	assert.Equal(t, volcengineSeedASRResourceID, headers.Get("X-Api-Resource-Id"))
}

func TestBuildVolcengineASRHeadersWithLegacyConsoleCredentials(t *testing.T) {
	headers, err := buildVolcengineASRHeaders("app-id|access-token|secret-key", volcengineSeedASRResourceID, "req-3")

	require.NoError(t, err)
	assert.Equal(t, "app-id", headers.Get("X-Api-App-Key"))
	assert.Equal(t, "access-token", headers.Get("X-Api-Access-Key"))
	assert.Empty(t, headers.Get("X-Api-Key"))
	assert.Equal(t, volcengineSeedASRResourceID, headers.Get("X-Api-Resource-Id"))
}

func TestConvertAudioRequestBuildsNativeASRSession(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "volc-asr-2"))
	require.NoError(t, writer.WriteField("language", "zh-CN"))
	require.NoError(t, writer.WriteField("response_format", "verbose_json"))
	require.NoError(t, writer.WriteField("resource_id", "volc.seedasr.sauc.concurrent"))
	require.NoError(t, writer.WriteField("show_utterances", "false"))
	require.NoError(t, writer.WriteField("metadata", `{"request":{"result_type":"single"},"chunk_size":4096}`))
	part, err := writer.CreateFormFile("file", "audio.wav")
	require.NoError(t, err)
	_, err = part.Write([]byte("audio-data"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/audio/transcriptions", &body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	info := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeAudioTranscription,
		OriginModelName: "volc-asr-2",
		IsStream:        true,
		RequestId:       "request-from-relay",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: channelconstant.ChannelBaseURLs[channelconstant.ChannelTypeVolcEngine],
		},
	}

	reader, err := (&Adaptor{}).ConvertAudioRequest(c, info, dto.AudioRequest{Model: "volc-asr-2"})

	require.NoError(t, err)
	assert.Nil(t, reader)
	assert.False(t, info.IsStream)
	value, exists := c.Get(contextKeyASRSession)
	require.True(t, exists)
	session, ok := value.(VolcengineASRSession)
	require.True(t, ok)
	assert.Equal(t, []byte("audio-data"), session.AudioData)
	assert.Equal(t, "wav", session.Request.Audio["format"])
	assert.Equal(t, "zh-CN", session.Request.Audio["language"])
	assert.Equal(t, volcengineASRDefaultSampleRate, session.Request.Audio["rate"])
	assert.Equal(t, "volc.seedasr.sauc.concurrent", session.ResourceID)
	assert.Equal(t, "request-from-relay", session.RequestID)
	assert.Equal(t, 4096, session.ChunkSize)
	assert.Equal(t, "single", session.Request.Request["result_type"])
	assert.Equal(t, false, session.Request.Request["show_utterances"])
}

func TestAudioOnlyRequestFrameUsesSequenceAndFinalFlag(t *testing.T) {
	frame, err := AudioOnlyRequestFrame([]byte("audio"), 2, true)
	require.NoError(t, err)

	msg, err := NewMessageFromBytes(frame)

	require.NoError(t, err)
	assert.Equal(t, MsgTypeAudioOnlyClient, msg.MsgType)
	assert.Equal(t, MsgTypeFlagNegativeSeq, msg.MsgTypeFlag)
	assert.Equal(t, SerializationNone, msg.Serialization)
	assert.Equal(t, CompressionNone, msg.Compression)
	assert.Equal(t, int32(-2), msg.Sequence)
	assert.Equal(t, []byte("audio"), msg.Payload)
}

func TestSendVolcengineASRAudioStartsAfterFullClientRequestSequence(t *testing.T) {
	type receiveResult struct {
		Messages []*Message
		Err      error
	}

	received := make(chan receiveResult, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			received <- receiveResult{Err: err}
			return
		}
		defer conn.Close()

		messages := make([]*Message, 0, 2)
		for range 2 {
			msg, err := ReceiveMessage(conn)
			if err != nil {
				received <- receiveResult{Err: err}
				return
			}
			messages = append(messages, msg)
		}
		received <- receiveResult{Messages: messages}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	require.NoError(t, sendVolcengineASRAudio(conn, []byte("abcdef"), 3))
	require.NoError(t, conn.Close())

	select {
	case result := <-received:
		require.NoError(t, result.Err)
		require.Len(t, result.Messages, 2)
		assert.Equal(t, int32(2), result.Messages[0].Sequence)
		assert.Equal(t, MsgTypeFlagPositiveSeq, result.Messages[0].MsgTypeFlag)
		assert.Equal(t, []byte("abc"), result.Messages[0].Payload)
		assert.Equal(t, int32(-3), result.Messages[1].Sequence)
		assert.Equal(t, MsgTypeFlagNegativeSeq, result.Messages[1].MsgTypeFlag)
		assert.Equal(t, []byte("def"), result.Messages[1].Payload)
	case <-time.After(2 * time.Second):
		require.Fail(t, "timed out waiting for websocket audio frames")
	}
}

func TestParseVolcengineASRResponsePayload(t *testing.T) {
	payload := []byte(`{
		"audio_info":{"duration":3696},
		"result":{
			"text":"这是字节跳动。",
			"utterances":[{"text":"这是字节跳动。","start_time":0,"end_time":3696,"definite":true}]
		}
	}`)

	result, ok, err := parseVolcengineASRResponsePayload(payload)

	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "这是字节跳动。", result.Text)
	assert.Equal(t, 3.696, result.Duration)
	require.Len(t, result.Utterances, 1)
	assert.Equal(t, 3696, result.Utterances[0].EndTime)
}

func TestVolcengineASRUsageFromDuration(t *testing.T) {
	info := &relaycommon.RelayInfo{}
	usage := volcengineASRUsage(info, 3.696)

	assert.Equal(t, 67, usage.PromptTokens)
	assert.Equal(t, 67, usage.TotalTokens)
	assert.Equal(t, 67, usage.PromptTokensDetails.AudioTokens)
}

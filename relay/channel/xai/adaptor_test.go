package xai

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetRequestURLForcesImageGenerationEndpoint(t *testing.T) {
	info := &relaycommon.RelayInfo{
		RelayMode:      relayconstant.RelayModeImagesGenerations,
		RequestURLPath: "/v1/chat/completions",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://example.com",
			ChannelType:    constant.ChannelTypeXai,
		},
	}

	requestURL, err := (&Adaptor{}).GetRequestURL(info)

	require.NoError(t, err)
	assert.Equal(t, "https://example.com/v1/images/generations", requestURL)
}

func TestGetRequestURLForcesSTTEndpoint(t *testing.T) {
	info := &relaycommon.RelayInfo{
		RelayMode:      relayconstant.RelayModeAudioTranscription,
		RequestURLPath: "/v1/audio/transcriptions",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://example.com",
			ChannelType:    constant.ChannelTypeXai,
		},
	}

	requestURL, err := (&Adaptor{}).GetRequestURL(info)

	require.NoError(t, err)
	assert.Equal(t, "https://example.com/v1/stt", requestURL)
}

func TestConvertAudioRequestOmitsRoutingModelForXAI(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "grok-stt"))
	require.NoError(t, writer.WriteField("language", "en"))
	require.NoError(t, writer.WriteField("url", "https://example.com/audio.mp3"))
	require.NoError(t, writer.Close())

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/audio/transcriptions", &body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())

	reader, err := (&Adaptor{}).ConvertAudioRequest(c, &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeAudioTranscription,
	}, dto.AudioRequest{Model: "grok-stt"})

	require.NoError(t, err)
	converted, err := io.ReadAll(reader)
	require.NoError(t, err)
	convertedBody := string(converted)
	assert.NotContains(t, convertedBody, `name="model"`)
	assert.Contains(t, convertedBody, `name="language"`)
	assert.Contains(t, convertedBody, `name="url"`)
	assert.True(t, strings.HasPrefix(c.Request.Header.Get("Content-Type"), "multipart/form-data; boundary="))
}

func TestConvertAudioRequestWritesFileLastForXAI(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	filePart, err := writer.CreateFormFile("file", "audio.mp3")
	require.NoError(t, err)
	_, err = filePart.Write([]byte("audio"))
	require.NoError(t, err)
	require.NoError(t, writer.WriteField("language", "en"))
	require.NoError(t, writer.WriteField("format", "true"))
	require.NoError(t, writer.Close())

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/audio/transcriptions", &body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())

	reader, err := (&Adaptor{}).ConvertAudioRequest(c, &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeAudioTranscription,
	}, dto.AudioRequest{Model: "grok-stt"})

	require.NoError(t, err)
	converted, err := io.ReadAll(reader)
	require.NoError(t, err)
	convertedBody := string(converted)
	languageIndex := strings.Index(convertedBody, `name="language"`)
	formatIndex := strings.Index(convertedBody, `name="format"`)
	fileIndex := strings.Index(convertedBody, `name="file"`)
	require.NotEqual(t, -1, languageIndex)
	require.NotEqual(t, -1, formatIndex)
	require.NotEqual(t, -1, fileIndex)
	assert.Greater(t, fileIndex, languageIndex)
	assert.Greater(t, fileIndex, formatIndex)
}

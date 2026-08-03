package zhipu_4v

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetRequestURLForAudioTranscription(t *testing.T) {
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeAudioTranscription,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://open.bigmodel.cn",
			ChannelType:    constant.ChannelTypeZhipu_v4,
		},
	}

	requestURL, err := (&Adaptor{}).GetRequestURL(info)

	require.NoError(t, err)
	assert.Equal(t, "https://open.bigmodel.cn/api/paas/v4/audio/transcriptions", requestURL)
}

func TestConvertAudioRequestAllowsFileBase64(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "glm-asr-2512"))
	require.NoError(t, writer.WriteField("file_base64", "AAAA"))
	require.NoError(t, writer.WriteField("stream", "true"))
	require.NoError(t, writer.Close())

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/audio/transcriptions", &body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())

	reader, err := (&Adaptor{}).ConvertAudioRequest(c, &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeAudioTranscription,
	}, dto.AudioRequest{Model: "glm-asr-2512"})

	require.NoError(t, err)
	converted, err := io.ReadAll(reader)
	require.NoError(t, err)
	convertedBody := string(converted)
	assert.Contains(t, convertedBody, `name="model"`)
	assert.Contains(t, convertedBody, "glm-asr-2512")
	assert.Contains(t, convertedBody, `name="file_base64"`)
	assert.Contains(t, convertedBody, `name="stream"`)
}

package xai

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestConvertImageEditRequestMultipart(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const originalImage = "reference image bytes"
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "grok-imagine-image-quality"))
	require.NoError(t, writer.WriteField("prompt", "keep the same character identity"))
	require.NoError(t, writer.WriteField("stream", "true"))
	require.NoError(t, writer.WriteField("response_format", "b64_json"))
	part, err := writer.CreateFormFile("image", "reference.png")
	require.NoError(t, err)
	_, err = part.Write([]byte(originalImage))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	originalContentType := writer.FormDataContentType()

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", &body)
	c.Request.Header.Set("Content-Type", originalContentType)
	storage, err := common.GetBodyStorage(c)
	require.NoError(t, err)
	c.Request.Body = io.NopCloser(storage)

	info := &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeImagesEdits}
	request := dto.ImageRequest{
		Model:          "grok-imagine-image-quality",
		Prompt:         "keep the same character identity",
		Stream:         common.GetPointer(true),
		ResponseFormat: "b64_json",
	}

	converted, err := (&Adaptor{}).ConvertImageRequest(c, info, request)
	require.NoError(t, err)
	convertedBody, ok := converted.(*bytes.Buffer)
	require.True(t, ok)
	require.NotEqual(t, originalContentType, c.Request.Header.Get("Content-Type"))

	replayed := httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(convertedBody.Bytes()))
	replayed.Header.Set("Content-Type", c.Request.Header.Get("Content-Type"))
	require.NoError(t, replayed.ParseMultipartForm(32<<20))
	require.Equal(t, "grok-imagine-image-quality", replayed.PostForm.Get("model"))
	require.Equal(t, "keep the same character identity", replayed.PostForm.Get("prompt"))
	require.Equal(t, "true", replayed.PostForm.Get("stream"))
	require.Equal(t, "b64_json", replayed.PostForm.Get("response_format"))
	require.Len(t, replayed.MultipartForm.File["image"], 1)

	file, err := replayed.MultipartForm.File["image"][0].Open()
	require.NoError(t, err)
	defer file.Close()
	fileBytes, err := io.ReadAll(file)
	require.NoError(t, err)
	require.Equal(t, []byte(originalImage), fileBytes)
}

package xai

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
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

func TestGrokImage2RequestPricing(t *testing.T) {
	expression, ok := billing_setting.GetBuiltinBillingExpr("grok-imagine-image-2.0")
	require.True(t, ok)
	for _, tc := range []struct {
		name, fields, tier string
		edit               bool
		quota              int
	}{
		{"default", ``, "1k_low", false, 20000},
		{"1k low", `,"resolution":"1k","quality":"low"`, "1k_low", false, 20000},
		{"1.5k low", `,"resolution":"1.5k","quality":"low"`, "1.5k_low", false, 25000},
		{"2k low", `,"resolution":"2k","quality":"low"`, "2k_low", false, 30000},
		{"1k medium", `,"resolution":"1k","quality":"medium"`, "1k_medium", false, 30000},
		{"1.5k medium", `,"resolution":"1.5k","quality":"medium"`, "1.5k_medium", false, 35000},
		{"2k medium", `,"resolution":"2k","quality":"medium"`, "2k_medium", false, 40000},
		{"size alias", `,"size":"2048x2048","quality":"medium","aspect_ratio":"16:9"`, "2k_medium", false, 40000},
		{"edit low", `,"quality":"low","image":{"url":"https://example.com/source.png","type":"image_url"}`, "1k_low", true, 25000},
		{"edit auto", `,"quality":"auto","image":{"file_id":"file-reference"}`, "1k_medium", true, 35000},
		{"edit default", `,"image":{"url":"https://example.com/source.png"}`, "1k_medium", true, 35000},
		{"multiple inputs and outputs", `,"n":2,"resolution":"2k","quality":"low","images":[{"url":"https://example.com/1.png"},{"file_id":"file-2"}]`, "2k_low", true, 70000},
		{"input count cannot be spoofed", `,"input_image_count":999`, "1k_low", false, 20000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mode, path := relayconstant.RelayModeImagesGenerations, "/v1/images/generations"
			if tc.edit {
				mode, path = relayconstant.RelayModeImagesEdits, "/v1/images/edits"
			}
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"model":"grok-imagine-image-2.0","prompt":"draw"`+tc.fields+`}`))
			c.Request.Header.Set("Content-Type", "application/json")
			request, err := helper.GetAndValidOpenAIImageRequest(c, mode)
			require.NoError(t, err)
			info := &relaycommon.RelayInfo{Request: request, RelayMode: mode}
			converted, err := (&Adaptor{}).ConvertImageRequest(c, info, *request)
			require.NoError(t, err)
			upstream := converted.(ImageRequest)
			require.NotNil(t, upstream.Resolution)
			assert.Equal(t, helper.ResolveGrokImageResolution(request), *upstream.Resolution)
			assert.Equal(t, request.Image, upstream.Image)
			assert.Equal(t, request.Images, upstream.Images)
			assert.Equal(t, request.AspectRatio, upstream.AspectRatio)
			require.NotNil(t, upstream.Quality)
			if request.Quality == "" || request.Quality == "auto" {
				if tc.edit {
					assert.Equal(t, "medium", *upstream.Quality)
				} else {
					assert.Equal(t, "low", *upstream.Quality)
				}
			} else {
				assert.Equal(t, request.Quality, *upstream.Quality)
			}
			input, err := helper.ResolveImageBillingRequestInput(c, info, billingexpr.RequestInput{})
			require.NoError(t, err)
			assert.NotContains(t, string(input.Body), "example.com")
			assert.NotContains(t, string(input.Body), "file-reference")
			snapshot := &billingexpr.BillingSnapshot{ExprString: expression, ExprHash: billingexpr.ExprHashString(expression), GroupRatio: 1, QuotaPerUnit: 500000}
			result, err := billingexpr.ComputeTieredQuotaWithRequest(snapshot, billingexpr.TokenParams{}, input)
			require.NoError(t, err)
			assert.Equal(t, tc.quota, result.ActualQuotaAfterGroup)
			assert.Equal(t, tc.tier, result.MatchedTier)
			if tc.name == "multiple inputs and outputs" {
				// 少返回一张输出时，只退输出费用，输入图片费用仍收一次。
				input.ImageCount = common.GetPointer(1)
				result, err = billingexpr.ComputeTieredQuotaWithRequest(snapshot, billingexpr.TokenParams{}, input)
				require.NoError(t, err)
				assert.Equal(t, 40000, result.ActualQuotaAfterGroup)
			}
		})
	}
}

func TestGrokImage2RejectsInvalidPricingParameters(t *testing.T) {
	for _, fields := range []string{
		`,"n":11`, `,"resolution":"4k"`, `,"quality":"high"`,
		`,"images":[{}]`,
		`,"image":{"url":"https://example.com/source.png"}`,
		`,"images":[{"url":"a"},{"url":"b"},{"url":"c"},{"url":"d"},{"url":"e"},{"url":"f"}]`,
	} {
		t.Run(fields, func(t *testing.T) {
			var request dto.ImageRequest
			require.NoError(t, common.Unmarshal([]byte(`{"model":"grok-imagine-image-2.0","prompt":"draw"`+fields+`}`), &request))
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
			c.Request.Header.Set("Content-Type", "application/json")
			_, err := (&Adaptor{}).ConvertImageRequest(c, &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeImagesGenerations}, request)
			require.Error(t, err)
		})
	}
}

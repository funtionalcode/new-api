package helper

import (
	"encoding/json"
	"fmt"
	"maps"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
)

func ResolveIncomingBillingExprRequestInput(c *gin.Context, info *relaycommon.RelayInfo) (billingexpr.RequestInput, error) {
	if info != nil && info.BillingRequestInput != nil {
		input := cloneRequestInput(*info.BillingRequestInput)
		merged := cloneStringMap(info.RequestHeaders)
		maps.Copy(merged, input.Headers)
		input.Headers = merged
		return input, nil
	}

	input := billingexpr.RequestInput{}
	if info != nil {
		input.Headers = cloneStringMap(info.RequestHeaders)
	}

	bodyBytes, err := readIncomingBillingExprBody(c)
	if err != nil {
		return billingexpr.RequestInput{}, err
	}
	input.Body = bodyBytes
	return input, nil
}

// ResolveImageBillingRequestInput freezes only the validated scalar image
// parameters needed by pricing. Image files, prompts and base64 payloads are
// deliberately excluded, including for multipart edits.
func ResolveImageBillingRequestInput(c *gin.Context, info *relaycommon.RelayInfo, input billingexpr.RequestInput) (billingexpr.RequestInput, error) {
	request, ok := info.Request.(*dto.ImageRequest)
	if !ok {
		return input, nil
	}
	count, err := request.ImageCount(false)
	if err != nil {
		return input, err
	}
	body := map[string]any{"model": request.Model, "n": count, "size": request.Size, "quality": request.Quality}
	if request.Model == "grok-imagine-image-2.0" {
		inputCount, err := ResolveImageInputCount(c, request)
		if err != nil {
			return input, err
		}
		body["resolution"] = ResolveGrokImageResolution(request)
		body["input_image_count"] = inputCount
		body["image_edit"] = info.RelayMode == relayconstant.RelayModeImagesEdits
	}
	if request.BillingParameters != nil {
		body["parameters"] = request.BillingParameters
	}
	encoded, err := common.Marshal(body)
	if err != nil {
		return input, err
	}
	input.Body = encoded
	input.ImageCount = &count
	return input, nil
}

// ResolveGrokImageResolution 对齐图片请求和按张计费使用的分辨率。
func ResolveGrokImageResolution(request *dto.ImageRequest) string {
	if request.Resolution != nil {
		return strings.ToLower(strings.TrimSpace(*request.Resolution))
	}
	switch strings.ToLower(request.Size) {
	case "1.5k", "1536x1536":
		return "1.5k"
	case "2k", "2048x2048":
		return "2k"
	default:
		return "1k"
	}
}

// ResolveImageInputCount 只保留输入图片数量，不把图片载荷写入计费快照。
func ResolveImageInputCount(c *gin.Context, request *dto.ImageRequest) (int, error) {
	count := 0
	var sources []json.RawMessage
	if len(request.Image) > 0 && strings.TrimSpace(string(request.Image)) != "null" {
		sources = append(sources, request.Image)
	}
	if len(request.Images) > 0 && strings.TrimSpace(string(request.Images)) != "null" {
		var images []json.RawMessage
		if err := common.Unmarshal(request.Images, &images); err != nil {
			return 0, fmt.Errorf("images must be an array: %w", err)
		}
		sources = append(sources, images...)
	}
	for _, raw := range sources {
		var source any
		if err := common.Unmarshal(raw, &source); err != nil {
			return 0, fmt.Errorf("invalid input image: %w", err)
		}
		valid := false
		switch image := source.(type) {
		case string:
			valid = strings.TrimSpace(image) != ""
		case map[string]any:
			for _, field := range []string{"url", "file_id"} {
				if value, ok := image[field].(string); ok && strings.TrimSpace(value) != "" {
					valid = true
				}
			}
		}
		if !valid {
			return 0, fmt.Errorf("input image must contain a URL or file_id")
		}
		count++
	}
	if c != nil && c.Request != nil && c.Request.MultipartForm != nil {
		for _, field := range []string{"image", "image[]", "images", "images[]"} {
			for _, file := range c.Request.MultipartForm.File[field] {
				if file.Size > 0 {
					count++
				}
			}
		}
	}
	if count > dto.MaxImageN {
		return 0, fmt.Errorf("too many input images: maximum %d", dto.MaxImageN)
	}
	return count, nil
}

func BuildBillingExprRequestInputFromRequest(request dto.Request, headers map[string]string) (billingexpr.RequestInput, error) {
	input := billingexpr.RequestInput{
		Headers: cloneStringMap(headers),
	}
	if request == nil {
		return input, nil
	}

	bodyBytes, err := common.Marshal(request)
	if err != nil {
		return billingexpr.RequestInput{}, err
	}
	input.Body = bodyBytes
	return input, nil
}

func readIncomingBillingExprBody(c *gin.Context) ([]byte, error) {
	if c == nil || c.Request == nil || !isJSONContentType(c.Request.Header.Get("Content-Type")) {
		return nil, nil
	}
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return nil, err
	}
	return storage.Bytes()
}

func cloneRequestInput(src billingexpr.RequestInput) billingexpr.RequestInput {
	input := billingexpr.RequestInput{
		Headers: cloneStringMap(src.Headers),
	}
	if src.ImageCount != nil {
		count := *src.ImageCount
		input.ImageCount = &count
	}
	if len(src.Body) > 0 {
		input.Body = append([]byte(nil), src.Body...)
	}
	return input
}

func isJSONContentType(contentType string) bool {
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	return strings.HasPrefix(contentType, "application/json")
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return map[string]string{}
	}
	dst := make(map[string]string, len(src))
	for key, value := range src {
		if strings.TrimSpace(key) == "" {
			continue
		}
		dst[key] = value
	}
	return dst
}

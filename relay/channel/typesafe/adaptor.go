package typesafe

import (
	"errors"
	"io"
	"math"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
)

var ModelList = []string{"jev-latest", "jev-preview", "jev-1.13.0"}

// Adaptor 复用公共 HTTP 传输，只允许 TypeSafe 原生评估接口。
type Adaptor struct{ openai.Adaptor }

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if info.RelayMode != relayconstant.RelayModeTypeSafe {
		return "", errors.New("TypeSafe only supports /v1/systemone")
	}
	base := strings.TrimRight(info.ChannelBaseUrl, "/")
	if base == "" {
		base = "https://api.typesafe.ai"
	}
	return strings.TrimSuffix(base, "/v1") + "/v1/systemone", nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, body io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, body)
}

func (a *Adaptor) DoResponse(c *gin.Context, response *http.Response, info *relaycommon.RelayInfo) (any, *types.NewAPIError) {
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeReadResponseBodyFailed)
	}
	usage, err := ParseUsage(body)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponse)
	}
	info.SetFirstResponseTime()
	c.Data(response.StatusCode, "application/json", body)
	return usage, nil
}

func ParseUsage(body []byte) (*dto.Usage, error) {
	var response struct {
		Model   string         `json:"model"`
		Answers map[string]any `json:"answers"`
		Usage   *struct {
			Input  *int64 `json:"input_tokens"`
			Output *int64 `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := common.Unmarshal(body, &response); err != nil {
		return nil, err
	}
	if response.Model == "" || len(response.Answers) == 0 || response.Usage == nil || response.Usage.Input == nil || response.Usage.Output == nil {
		return nil, errors.New("invalid TypeSafe response: model, answers, and token usage are required")
	}
	input, output := *response.Usage.Input, *response.Usage.Output
	if input < 0 || output < 0 || input > math.MaxInt32 || output > math.MaxInt32-input {
		return nil, errors.New("invalid TypeSafe token usage")
	}
	return &dto.Usage{PromptTokens: int(input), CompletionTokens: int(output), TotalTokens: int(input + output)}, nil
}

func (a *Adaptor) GetModelList() []string { return ModelList }
func (a *Adaptor) GetChannelName() string { return "TypeSafe" }

package grok

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

type Adaptor struct{}

func (a *Adaptor) Init(*relaycommon.RelayInfo) {}

func (a *Adaptor) ConvertGeminiRequest(*gin.Context, *relaycommon.RelayInfo, *dto.GeminiChatRequest) (any, error) {
	return nil, errors.New("grok channel: endpoint not supported")
}

func (a *Adaptor) ConvertClaudeRequest(*gin.Context, *relaycommon.RelayInfo, *dto.ClaudeRequest) (any, error) {
	return nil, errors.New("grok channel: /v1/messages endpoint not supported")
}

func (a *Adaptor) ConvertAudioRequest(*gin.Context, *relaycommon.RelayInfo, dto.AudioRequest) (io.Reader, error) {
	return nil, errors.New("grok channel: endpoint not supported")
}

func (a *Adaptor) ConvertImageRequest(*gin.Context, *relaycommon.RelayInfo, dto.ImageRequest) (any, error) {
	return nil, errors.New("grok channel: endpoint not supported")
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	return (&openai.Adaptor{}).ConvertOpenAIRequest(c, info, request)
}

func (a *Adaptor) ConvertRerankRequest(*gin.Context, int, dto.RerankRequest) (any, error) {
	return nil, errors.New("grok channel: endpoint not supported")
}

func (a *Adaptor) ConvertEmbeddingRequest(*gin.Context, *relaycommon.RelayInfo, dto.EmbeddingRequest) (any, error) {
	return nil, errors.New("grok channel: endpoint not supported")
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	return (&openai.Adaptor{}).ConvertOpenAIResponsesRequest(c, info, request)
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if info == nil {
		return "", errors.New("grok channel: missing relay info")
	}
	path := ""
	switch info.RelayMode {
	case relayconstant.RelayModeChatCompletions:
		path = "/v1/chat/completions"
	case relayconstant.RelayModeResponses:
		path = "/v1/responses"
	default:
		return "", errors.New("grok channel: only /v1/chat/completions and /v1/responses are supported")
	}
	return relaycommon.GetFullRequestURL(info.ChannelBaseUrl, path, info.ChannelType), nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	if info == nil {
		return errors.New("grok channel: missing relay info")
	}
	credential, err := dto.ParseGrokCredential(info.ApiKey)
	if err != nil {
		return err
	}
	accessToken := strings.TrimSpace(credential.AccessToken)
	if accessToken == "" {
		return errors.New("grok channel: access_token is required")
	}

	channel.SetupApiRequestHeader(info, c, req)
	setCLIIdentityHeaders(*req, accessToken)
	req.Set("Content-Type", "application/json")
	if info.IsStream {
		req.Set("Accept", "text/event-stream")
	} else if req.Get("Accept") == "" {
		req.Set("Accept", "application/json")
	}
	return nil
}

func setCLIIdentityHeaders(header http.Header, accessToken string) {
	header.Set("Authorization", "Bearer "+accessToken)
	header.Set("X-XAI-Token-Auth", "xai-grok-cli")
	header.Set("X-Grok-Client-Version", DefaultCLIClientVersion)
	header.Set("User-Agent", "xai-grok-workspace/"+DefaultCLIClientVersion)
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (any, *types.NewAPIError) {
	if info == nil || (info.RelayMode != relayconstant.RelayModeChatCompletions && info.RelayMode != relayconstant.RelayModeResponses) {
		return nil, types.NewError(errors.New("grok channel: endpoint not supported"), types.ErrorCodeInvalidRequest)
	}
	if info.RelayMode == relayconstant.RelayModeChatCompletions {
		return (&openai.Adaptor{}).DoResponse(c, resp, info)
	}
	if info.IsStream {
		return openai.OaiResponsesStreamHandler(c, info, resp)
	}
	return openai.OaiResponsesHandler(c, info, resp)
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

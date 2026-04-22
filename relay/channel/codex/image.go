package codex

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

const (
	codexImageGenerationToolType     = "image_generation"
	codexDefaultImageToolModel       = "gpt-image-2"
	codexDefaultImageResponsesModel  = "gpt-5.4-mini"
	codexResponsesInputRoleUser      = "user"
	codexResponsesInputTypeInputText = "input_text"
	codexImageStreamMaxBufferSize    = 64 << 20
)

var (
	codexDefaultImageBackground        = json.RawMessage(`"auto"`)
	codexDefaultImageModeration        = json.RawMessage(`"auto"`)
	codexDefaultImageOutputCompression = json.RawMessage(`100`)
	codexDefaultImageOutputFormat      = json.RawMessage(`"png"`)
	codexImageResponsesInclude         = json.RawMessage(`["reasoning.encrypted_content"]`)
)

type codexResponsesInputItem struct {
	Role    string                       `json:"role"`
	Content []codexResponsesInputContent `json:"content"`
}

type codexResponsesInputContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type codexImageGenerationTool struct {
	Type              string          `json:"type"`
	Background        json.RawMessage `json:"background,omitempty"`
	Model             string          `json:"model,omitempty"`
	Moderation        json.RawMessage `json:"moderation,omitempty"`
	N                 *uint           `json:"n,omitempty"`
	OutputCompression json.RawMessage `json:"output_compression,omitempty"`
	OutputFormat      json.RawMessage `json:"output_format,omitempty"`
	Quality           string          `json:"quality,omitempty"`
	Size              string          `json:"size,omitempty"`
	PartialImages     json.RawMessage `json:"partial_images,omitempty"`
}

type codexImageGenerationResponse struct {
	ID        string                       `json:"id"`
	Object    string                       `json:"object"`
	CreatedAt int64                        `json:"created_at"`
	Error     any                          `json:"error,omitempty"`
	Output    []codexImageGenerationOutput `json:"output"`
	Usage     *dto.Usage                   `json:"usage"`
}

type codexImageGenerationOutput struct {
	Type   string          `json:"type"`
	Result json.RawMessage `json:"result"`
}

type codexImageGenerationStreamEvent struct {
	Type     string                        `json:"type"`
	Response *codexImageGenerationResponse `json:"response,omitempty"`
	Item     *codexImageGenerationOutput   `json:"item,omitempty"`
	Error    any                           `json:"error,omitempty"`
}

func convertCodexImageRequest(info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	if info == nil || info.RelayMode != relayconstant.RelayModeImagesGenerations {
		return nil, errors.New("codex channel: only /v1/images/generations is supported for image requests")
	}
	info.IsStream = true

	input, err := common.Marshal([]codexResponsesInputItem{
		{
			Role: codexResponsesInputRoleUser,
			Content: []codexResponsesInputContent{
				{
					Type: codexResponsesInputTypeInputText,
					Text: request.Prompt,
				},
			},
		},
	})
	if err != nil {
		return nil, err
	}

	tools, err := common.Marshal([]codexImageGenerationTool{newCodexImageGenerationTool(info, request)})
	if err != nil {
		return nil, err
	}

	return dto.OpenAIResponsesRequest{
		Model:        codexImageResponsesModel(info, request),
		Input:        input,
		Include:      codexImageResponsesInclude,
		Instructions: json.RawMessage(`""`),
		Store:        json.RawMessage(`false`),
		Stream:       common.GetPointer(true),
		Tools:        tools,
		User:         request.User,
	}, nil
}

func newCodexImageGenerationTool(info *relaycommon.RelayInfo, request dto.ImageRequest) codexImageGenerationTool {
	n := request.N
	if n == nil {
		n = common.GetPointer(uint(1))
	}

	return codexImageGenerationTool{
		Type:              codexImageGenerationToolType,
		Background:        rawOrDefault(request.Background, codexDefaultImageBackground),
		Model:             codexImageToolModel(info, request),
		Moderation:        rawOrDefault(request.Moderation, codexDefaultImageModeration),
		N:                 n,
		OutputCompression: rawOrDefault(request.OutputCompression, codexDefaultImageOutputCompression),
		OutputFormat:      rawOrDefault(request.OutputFormat, codexDefaultImageOutputFormat),
		Quality:           stringOrDefault(request.Quality, "auto"),
		Size:              stringOrDefault(request.Size, "auto"),
		PartialImages:     request.PartialImages,
	}
}

func codexImageResponsesModel(info *relaycommon.RelayInfo, request dto.ImageRequest) string {
	if !isCodexImageToolModel(request.Model) && strings.TrimSpace(request.Model) != "" {
		return strings.TrimSpace(request.Model)
	}
	if info != nil && !isCodexImageToolModel(info.UpstreamModelName) && strings.TrimSpace(info.UpstreamModelName) != "" {
		return strings.TrimSpace(info.UpstreamModelName)
	}
	return codexDefaultImageResponsesModel
}

func codexImageToolModel(info *relaycommon.RelayInfo, request dto.ImageRequest) string {
	if info != nil && isCodexImageToolModel(info.OriginModelName) {
		return strings.TrimSpace(info.OriginModelName)
	}
	if isCodexImageToolModel(request.Model) {
		return strings.TrimSpace(request.Model)
	}
	return codexDefaultImageToolModel
}

func isCodexImageToolModel(model string) bool {
	model = strings.TrimSpace(strings.ToLower(model))
	return strings.HasPrefix(model, "gpt-image-") || strings.HasPrefix(model, "dall-e")
}

func rawOrDefault(value json.RawMessage, fallback json.RawMessage) json.RawMessage {
	if len(value) == 0 {
		return fallback
	}
	return value
}

func stringOrDefault(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func codexImageGenerationHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	codexResponse, err := readCodexImageGenerationStream(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	if oaiError := dto.GetOpenAIError(codexResponse.Error); oaiError != nil && oaiError.Type != "" {
		return nil, types.WithOpenAIError(*oaiError, resp.StatusCode)
	}

	imageResponse, err := responseCodex2OpenAIImage(&codexResponse, info)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	if len(imageResponse.Data) == 0 {
		return nil, types.NewOpenAIError(errors.New("codex channel: no image generation result"), types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	if info != nil {
		info.PriceData.AddOtherRatio("n", float64(len(imageResponse.Data)))
	}

	jsonResponse, err := common.Marshal(imageResponse)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	resp.Header.Set("Content-Type", "application/json")
	service.IOCopyBytesGracefully(c, resp, jsonResponse)

	return codexImageUsage(&codexResponse), nil
}

func readCodexImageGenerationStream(reader io.Reader) (codexImageGenerationResponse, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64<<10), codexImageStreamMaxBufferSize)

	response := codexImageGenerationResponse{}
	received := false

	for scanner.Scan() {
		data := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(data, "data:") {
			continue
		}
		data = strings.TrimSpace(strings.TrimPrefix(data, "data:"))
		if data == "" {
			continue
		}
		if strings.HasPrefix(data, "[DONE]") {
			break
		}

		var event codexImageGenerationStreamEvent
		if err := common.UnmarshalJsonStr(data, &event); err != nil {
			return codexImageGenerationResponse{}, err
		}
		if event.Error != nil {
			response.Error = event.Error
			received = true
		}
		if event.Item != nil && event.Item.Type == dto.ResponsesOutputTypeImageGenerationCall {
			response.Output = append(response.Output, *event.Item)
			received = true
		}
		if event.Response != nil {
			previousOutput := response.Output
			response = *event.Response
			if len(response.Output) == 0 {
				response.Output = previousOutput
			}
			received = true
		}
	}

	if err := scanner.Err(); err != nil {
		return codexImageGenerationResponse{}, err
	}
	if !received {
		return codexImageGenerationResponse{}, errors.New("codex channel: empty image generation stream")
	}
	return response, nil
}

func responseCodex2OpenAIImage(response *codexImageGenerationResponse, info *relaycommon.RelayInfo) (*dto.ImageResponse, error) {
	imageResponse := &dto.ImageResponse{
		Created: codexImageCreatedAt(response, info),
	}

	for _, output := range response.Output {
		if output.Type != dto.ResponsesOutputTypeImageGenerationCall {
			continue
		}

		items, err := codexImageDataFromResult(output.Result)
		if err != nil {
			return nil, err
		}
		imageResponse.Data = append(imageResponse.Data, items...)
	}

	return imageResponse, nil
}

func codexImageDataFromResult(result json.RawMessage) ([]dto.ImageData, error) {
	if isEmptyJSON(result) {
		return nil, nil
	}

	var b64 string
	if err := common.Unmarshal(result, &b64); err == nil {
		if b64 == "" {
			return nil, nil
		}
		return []dto.ImageData{{B64Json: b64}}, nil
	}

	var b64Items []string
	if err := common.Unmarshal(result, &b64Items); err == nil {
		return lo.FilterMap(b64Items, func(item string, _ int) (dto.ImageData, bool) {
			if item == "" {
				return dto.ImageData{}, false
			}
			return dto.ImageData{B64Json: item}, true
		}), nil
	}

	return nil, fmt.Errorf("codex channel: unsupported image result format")
}

func codexImageCreatedAt(response *codexImageGenerationResponse, info *relaycommon.RelayInfo) int64 {
	if response != nil && response.CreatedAt > 0 {
		return response.CreatedAt
	}
	if info != nil && !info.StartTime.IsZero() {
		return info.StartTime.Unix()
	}
	return time.Now().Unix()
}

func codexImageUsage(response *codexImageGenerationResponse) *dto.Usage {
	if response == nil || response.Usage == nil {
		return &dto.Usage{}
	}

	usage := *response.Usage
	if usage.PromptTokens == 0 {
		usage.PromptTokens = usage.InputTokens
	}
	if usage.CompletionTokens == 0 {
		usage.CompletionTokens = usage.OutputTokens
	}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}
	return &usage
}

func isEmptyJSON(data json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(data))
	return trimmed == "" || trimmed == "null"
}

package codex

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestConvertImageRequestBuildsCodexResponsesToolCall(t *testing.T) {
	t.Parallel()

	n := uint(2)
	request := dto.ImageRequest{
		Model:             "gpt-image-2",
		Prompt:            "draw a minimal app icon",
		N:                 &n,
		Quality:           "high",
		Background:        json.RawMessage(`"transparent"`),
		OutputCompression: json.RawMessage(`0`),
	}
	info := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeImagesGenerations,
		OriginModelName: "gpt-image-2",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gpt-image-2",
		},
	}

	got, err := (&Adaptor{}).ConvertImageRequest(gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New()), info, request)
	require.NoError(t, err)

	responsesRequest, ok := got.(dto.OpenAIResponsesRequest)
	require.True(t, ok)
	require.Equal(t, codexDefaultImageResponsesModel, responsesRequest.Model)
	require.JSONEq(t, `["reasoning.encrypted_content"]`, string(responsesRequest.Include))
	require.JSONEq(t, `false`, string(responsesRequest.Store))
	require.NotNil(t, responsesRequest.Stream)
	require.True(t, *responsesRequest.Stream)
	require.True(t, info.IsStream)

	var input []map[string]any
	require.NoError(t, common.Unmarshal(responsesRequest.Input, &input))
	require.Len(t, input, 1)
	require.Equal(t, "user", input[0]["role"])

	var tools []map[string]any
	require.NoError(t, common.Unmarshal(responsesRequest.Tools, &tools))
	require.Len(t, tools, 1)
	require.Equal(t, "image_generation", tools[0]["type"])
	require.Equal(t, "gpt-image-2", tools[0]["model"])
	require.Equal(t, "transparent", tools[0]["background"])
	require.Equal(t, "high", tools[0]["quality"])
	require.Equal(t, "auto", tools[0]["size"])
	require.Equal(t, "png", tools[0]["output_format"])
	require.Equal(t, float64(0), tools[0]["output_compression"])
	require.Equal(t, float64(2), tools[0]["n"])
}

func TestConvertImageRequestKeepsMappedResponsesModelAndOriginalToolModel(t *testing.T) {
	t.Parallel()

	request := dto.ImageRequest{
		Model:  "gpt-5.4",
		Prompt: "draw a minimal app icon",
	}
	info := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeImagesGenerations,
		OriginModelName: "gpt-image-2",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gpt-5.4",
			IsModelMapped:     true,
		},
	}

	got, err := (&Adaptor{}).ConvertImageRequest(gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New()), info, request)
	require.NoError(t, err)

	responsesRequest := got.(dto.OpenAIResponsesRequest)
	require.Equal(t, "gpt-5.4", responsesRequest.Model)

	var tools []map[string]any
	require.NoError(t, common.Unmarshal(responsesRequest.Tools, &tools))
	require.Len(t, tools, 1)
	require.Equal(t, "gpt-image-2", tools[0]["model"])
	require.Equal(t, float64(1), tools[0]["n"])
}

func TestDoResponseForImageGenerationConvertsStreamToolResultToImageResponse(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeImagesGenerations,
		StartTime: time.Unix(1699999999, 0),
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: ioNopCloser(strings.Join([]string{
			`data: {"type":"response.created","response":{"id":"resp_test","object":"response","created_at":1700000000}}`,
			`data: {"type":"response.output_item.done","item":{"type":"image_generation_call","result":"base64-stream-image"}}`,
			`data: {"type":"response.completed","response":{"id":"resp_test","object":"response","created_at":1700000000,"usage":{"input_tokens":13,"output_tokens":5,"total_tokens":18}}}`,
			`data: [DONE]`,
			``,
		}, "\n")),
	}
	resp.Header.Set("Content-Type", "text/event-stream")

	usageAny, apiErr := (&Adaptor{}).DoResponse(c, resp, info)
	require.Nil(t, apiErr)

	usage := usageAny.(*dto.Usage)
	require.Equal(t, 13, usage.PromptTokens)
	require.Equal(t, 5, usage.CompletionTokens)
	require.Equal(t, 18, usage.TotalTokens)
	require.Equal(t, "application/json", recorder.Header().Get("Content-Type"))

	var imageResponse dto.ImageResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &imageResponse))
	require.Equal(t, int64(1700000000), imageResponse.Created)
	require.Len(t, imageResponse.Data, 1)
	require.Equal(t, "base64-stream-image", imageResponse.Data[0].B64Json)
}

func TestCodexModelListIncludesNativeImageModels(t *testing.T) {
	t.Parallel()

	require.Contains(t, ModelList, "gpt-image-1")
	require.Contains(t, ModelList, "gpt-image-2")
	require.Contains(t, ModelList, "gpt-5.4-mini")
	require.NotContains(t, ModelList, "gpt-image-2-openai-compact")
}

type nopReadCloser struct {
	*strings.Reader
}

func (n nopReadCloser) Close() error {
	return nil
}

func ioNopCloser(body string) nopReadCloser {
	return nopReadCloser{Reader: strings.NewReader(body)}
}

package common

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPrepareXAIResponsesRequestNormalizesCodexCompatibility(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	request := dto.OpenAIResponsesRequest{
		Model: "grok-4.5",
		Tools: []byte(`[
			{"type":"function","name":"plain","strict":false,"external_web_access":true,"parameters":{"type":"object"}},
			{"type":"namespace","name":"mcp__browser","description":"browser tools","tools":[
				{"type":"function","name":"navigate","strict":false,"parameters":{"type":"object","properties":{"url":{"type":"string"}}}}
			]},
			{"type":"web_search","external_web_access":true}
		]`),
		Input: []byte(`[
			{"type":"function_call_output","call_id":"call_image","output":[
				{"type":"input_text","text":"tool image"},
				{"type":"input_image","image_url":"data:image/png;base64,abc","detail":"auto"}
			]},
			{"type":"function_call_output","call_id":"call_source","output":[
				{"type":"input_image","source":{"type":"base64","media_type":"image/jpeg","data":"xyz"}}
			]},
			{"type":"function_call_output","call_id":"call_text","output":"already valid"}
		]`),
	}

	got, err := PrepareXAIResponsesRequest(c, &RelayInfo{ChannelMeta: &ChannelMeta{ChannelType: constant.ChannelTypeXai}}, request)
	require.NoError(t, err)
	require.JSONEq(t, `[
		{"type":"function","name":"plain","parameters":{"type":"object"}},
		{"type":"function","name":"mcp__browser__navigate","parameters":{"type":"object","properties":{"url":{"type":"string"}}}},
		{"type":"web_search"},
		{"type":"x_search"}
	]`, string(got.Tools))
	require.JSONEq(t, `[
		{"type":"function_call_output","call_id":"call_image","output":"tool image"},
		{"type":"function_call_output","call_id":"call_source","output":"(see attached image)"},
		{"type":"function_call_output","call_id":"call_text","output":"already valid"},
		{"type":"message","role":"user","content":[
			{"type":"input_text","text":"Attached image(s) from tool result:"},
			{"type":"input_image","image_url":"data:image/png;base64,abc","detail":"auto"},
			{"type":"input_image","image_url":"data:image/jpeg;base64,xyz"}
		]}
	]`, string(got.Input))

	response, err := NormalizeXAIResponsesResponse(c, &RelayInfo{ChannelMeta: &ChannelMeta{ChannelType: constant.ChannelTypeXai}}, []byte(`{
		"type":"response.output_item.done",
		"item":{"type":"function_call","name":"mcp__browser__navigate","arguments":"{}","call_id":"call_1"}
	}`))
	require.NoError(t, err)
	require.JSONEq(t, `{
		"type":"response.output_item.done",
		"item":{"type":"function_call","namespace":"mcp__browser","name":"navigate","arguments":"{}","call_id":"call_1"}
	}`, string(response))
}

func TestPrepareXAIResponsesRequestAppliesToSuperGrokOnly(t *testing.T) {
	for _, channelType := range []int{constant.ChannelTypeXai, constant.ChannelTypeGrok} {
		t.Run(constant.GetChannelTypeName(channelType), func(t *testing.T) {
			request := dto.OpenAIResponsesRequest{Tools: []byte(`[{"type":"web_search","external_web_access":true}]`)}
			got, err := PrepareXAIResponsesRequest(nil, &RelayInfo{ChannelMeta: &ChannelMeta{ChannelType: channelType}}, request)
			require.NoError(t, err)
			require.JSONEq(t, `[{"type":"web_search"},{"type":"x_search"}]`, string(got.Tools))
		})
	}

	request := dto.OpenAIResponsesRequest{Tools: []byte(`[{"type":"web_search","external_web_access":true}]`)}
	got, err := PrepareXAIResponsesRequest(nil, &RelayInfo{ChannelMeta: &ChannelMeta{ChannelType: constant.ChannelTypeOpenAI}}, request)
	require.NoError(t, err)
	require.JSONEq(t, string(request.Tools), string(got.Tools))
}

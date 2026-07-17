package common

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPrepareXAIResponsesRequestNormalizesCodexCompatibility(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	request := dto.OpenAIResponsesRequest{
		Model:      "grok-4.5",
		ToolChoice: []byte(`{"type":"function","namespace":"mcp__browser","name":"navigate"}`),
		Tools: []byte(`[
			{"type":"function","name":"plain","strict":false,"external_web_access":true,"parameters":{"type":"object"}},
			{"type":"namespace","name":"mcp__browser","description":"browser tools","tools":[
				{"type":"function","name":"navigate","strict":false,"parameters":{"type":"object","properties":{"url":{"type":"string"}}}}
			]},
			{"type":"web_search","external_web_access":true}
		]`),
		Input: []byte(`[
			{"type":"function_call","namespace":"mcp__browser","name":"navigate","arguments":"{\"url\":\"https://example.com\"}","call_id":"call_namespace"},
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
	require.JSONEq(t, `{"type":"function","name":"mcp__browser__navigate"}`, string(got.ToolChoice))
	require.JSONEq(t, `[
		{"type":"function_call","name":"mcp__browser__navigate","arguments":"{\"url\":\"https://example.com\"}","call_id":"call_namespace"},
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

func TestPrepareXAIResponsesRequestNormalizesCustomToolRoundTrip(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	parallelToolCalls := json.RawMessage(`true`)
	request := dto.OpenAIResponsesRequest{
		Tools: json.RawMessage(`[
			{"type":"custom","name":"apply_patch","description":"Apply a patch.","format":{"type":"grammar","syntax":"lark","definition":"start: patch"}}
		]`),
		ToolChoice:        json.RawMessage(`{"type":"custom","name":"apply_patch"}`),
		ParallelToolCalls: parallelToolCalls,
		Input: json.RawMessage(`[
			{"type":"custom_tool_call","call_id":"call_old","name":"apply_patch","input":"*** Begin Patch\n*** End Patch"},
			{"type":"custom_tool_call_output","call_id":"call_old","name":"apply_patch","output":"Done"}
		]`),
	}

	got, err := PrepareXAIResponsesRequest(c, &RelayInfo{ChannelMeta: &ChannelMeta{ChannelType: constant.ChannelTypeXai}}, request)
	require.NoError(t, err)
	require.JSONEq(t, `[
		{"type":"function","name":"apply_patch","description":"Apply a patch. Pass the complete freeform input in the input field.","parameters":{
			"type":"object",
			"properties":{"input":{"type":"string","description":"Complete raw input for this tool."}},
			"required":["input"],
			"additionalProperties":false
		}}
	]`, string(got.Tools))
	require.JSONEq(t, `{"type":"function","name":"apply_patch"}`, string(got.ToolChoice))
	require.JSONEq(t, `[
		{"type":"function_call","call_id":"call_old","name":"apply_patch","arguments":"{\"input\":\"*** Begin Patch\\n*** End Patch\"}"},
		{"type":"function_call_output","call_id":"call_old","output":"Done"}
	]`, string(got.Input))

	added, err := NormalizeXAIResponsesResponse(c, &RelayInfo{ChannelMeta: &ChannelMeta{ChannelType: constant.ChannelTypeXai}}, []byte(`{
		"type":"response.output_item.added",
		"item":{"id":"fc_new","type":"function_call","call_id":"call_new","name":"apply_patch","arguments":""}
	}`))
	require.NoError(t, err)
	require.JSONEq(t, `{
		"type":"response.output_item.added",
		"item":{"id":"fc_new","type":"custom_tool_call","call_id":"call_new","name":"apply_patch","input":""}
	}`, string(added))

	delta, err := NormalizeXAIResponsesResponse(c, &RelayInfo{ChannelMeta: &ChannelMeta{ChannelType: constant.ChannelTypeXai}}, []byte(`{
		"type":"response.function_call_arguments.delta","item_id":"fc_new","call_id":"call_new","delta":"*** Begin Patch"
	}`))
	require.NoError(t, err)
	require.JSONEq(t, `{
		"type":"response.custom_tool_call_input.delta","item_id":"fc_new","call_id":"call_new","delta":"*** Begin Patch"
	}`, string(delta))

	done, err := NormalizeXAIResponsesResponse(c, &RelayInfo{ChannelMeta: &ChannelMeta{ChannelType: constant.ChannelTypeXai}}, []byte(`{
		"type":"response.output_item.done",
		"item":{"id":"fc_new","type":"function_call","call_id":"call_new","name":"apply_patch","arguments":"{\"input\":\"*** Begin Patch\\n*** End Patch\"}"}
	}`))
	require.NoError(t, err)
	require.JSONEq(t, `{
		"type":"response.output_item.done",
		"item":{"id":"fc_new","type":"custom_tool_call","call_id":"call_new","name":"apply_patch","input":"*** Begin Patch\n*** End Patch"}
	}`, string(done))

	empty, err := PrepareXAIResponsesRequest(nil, &RelayInfo{ChannelMeta: &ChannelMeta{ChannelType: constant.ChannelTypeGrok}}, dto.OpenAIResponsesRequest{
		Tools:             json.RawMessage(`[{"type":"tool_search"}]`),
		ToolChoice:        json.RawMessage(`"auto"`),
		ParallelToolCalls: parallelToolCalls,
	})
	require.NoError(t, err)
	require.Empty(t, empty.Tools)
	require.Empty(t, empty.ToolChoice)
	require.Empty(t, empty.ParallelToolCalls)
}

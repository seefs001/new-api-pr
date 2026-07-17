package grok

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
)

func TestConvertOpenAIRequestPreservesStreamUsageOption(t *testing.T) {
	stream := true
	request := &dto.GeneralOpenAIRequest{
		Model:         "grok-4.5",
		Stream:        &stream,
		StreamOptions: &dto.StreamOptions{IncludeUsage: true},
	}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ChannelType:          constant.ChannelTypeGrok,
		SupportStreamOptions: true,
	}}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, info, request)

	require.NoError(t, err)
	require.Same(t, request, converted)
	require.NotNil(t, request.StreamOptions)
	require.True(t, request.StreamOptions.IncludeUsage)
}

func TestConvertOpenAIResponsesRequestAppliesXAICompatibility(t *testing.T) {
	request := dto.OpenAIResponsesRequest{Tools: []byte(`[{"type":"web_search","external_web_access":true}]`)}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeGrok}}

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(nil, info, request)

	require.NoError(t, err)
	responsesRequest, ok := converted.(dto.OpenAIResponsesRequest)
	require.True(t, ok)
	require.JSONEq(t, `[{"type":"web_search"},{"type":"x_search"}]`, string(responsesRequest.Tools))
}

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

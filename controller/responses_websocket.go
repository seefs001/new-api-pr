package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type responsesWebSocketCreateEvent struct {
	Type     string          `json:"type"`
	Response json.RawMessage `json:"response"`
}

type responsesWebSocketState struct {
	mu            sync.Mutex
	clientWriteMu sync.Mutex
	inFlight      bool
	billing       relaycommon.BillingSettler
}

var responsesWebSocketUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var responsesWebSocketChannelTypes = []int{
	constant.ChannelTypeResponsesWS,
	constant.ChannelTypeCodex,
}

func RelayResponsesWebSocket(c *gin.Context) {
	clientConn, err := responsesWebSocketUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.LogError(c, "responses websocket upgrade failed: "+err.Error())
		return
	}
	defer clientConn.Close()

	messageType, message, err := readResponsesWebSocketTextMessage(clientConn)
	if err != nil {
		writeResponsesWebSocketError(c, clientConn, types.NewError(err, types.ErrorCodeInvalidRequest).ToOpenAIError())
		return
	}

	createEvent, request, err := parseResponsesWebSocketCreateEvent(message)
	if err != nil {
		writeResponsesWebSocketError(c, clientConn, types.NewError(err, types.ErrorCodeInvalidRequest).ToOpenAIError())
		return
	}
	if request.Model == "" {
		writeResponsesWebSocketError(c, clientConn, types.NewError(errors.New("model is required"), types.ErrorCodeInvalidRequest).ToOpenAIError())
		return
	}
	if newAPIError := ensureResponsesWebSocketModelAccess(c, request.Model); newAPIError != nil {
		writeResponsesWebSocketError(c, clientConn, newAPIError.ToOpenAIError())
		return
	}

	channel, newAPIError := selectResponsesWebSocketChannel(c, request.Model)
	if newAPIError != nil {
		writeResponsesWebSocketError(c, clientConn, newAPIError.ToOpenAIError())
		return
	}
	if setupErr := middleware.SetupContextForSelectedChannel(c, channel, request.Model); setupErr != nil {
		writeResponsesWebSocketError(c, clientConn, setupErr.ToOpenAIError())
		return
	}

	info, err := relaycommon.GenRelayInfo(c, types.RelayFormatOpenAIResponsesWS, request, clientConn)
	if err != nil {
		writeResponsesWebSocketError(c, clientConn, types.NewError(err, types.ErrorCodeGenRelayInfoFailed).ToOpenAIError())
		return
	}
	info.InitChannelMeta(c)

	state := &responsesWebSocketState{}
	upstreamMessage, newAPIError := prepareResponsesWebSocketCreateMessage(c, info, createEvent, request)
	if newAPIError != nil {
		writeResponsesWebSocketError(c, clientConn, newAPIError.ToOpenAIError())
		return
	}
	if newAPIError = preConsumeResponsesWebSocketRequest(c, info, request); newAPIError != nil {
		writeResponsesWebSocketError(c, clientConn, newAPIError.ToOpenAIError())
		return
	}
	state.begin(info.Billing)

	targetConn, err := dialResponsesWebSocket(c, info)
	if err != nil {
		state.refund(c)
		writeResponsesWebSocketError(c, clientConn, types.NewError(err, types.ErrorCodeDoRequestFailed).ToOpenAIError())
		return
	}
	info.TargetWs = targetConn
	defer targetConn.Close()

	if err := targetConn.WriteMessage(messageType, upstreamMessage); err != nil {
		state.refund(c)
		writeResponsesWebSocketError(c, clientConn, types.NewError(err, types.ErrorCodeDoRequestFailed).ToOpenAIError())
		return
	}

	forwardResponsesWebSocket(c, info, state)
}

func readResponsesWebSocketTextMessage(conn *websocket.Conn) (int, []byte, error) {
	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			return 0, nil, err
		}
		if messageType != websocket.TextMessage {
			return 0, nil, fmt.Errorf("responses websocket only accepts JSON text messages")
		}
		if strings.TrimSpace(string(message)) == "" {
			continue
		}
		return messageType, message, nil
	}
}

func parseResponsesWebSocketCreateEvent(message []byte) (map[string]json.RawMessage, *dto.OpenAIResponsesRequest, error) {
	var event responsesWebSocketCreateEvent
	if err := common.Unmarshal(message, &event); err != nil {
		return nil, nil, err
	}
	if event.Type != "response.create" {
		return nil, nil, fmt.Errorf("first responses websocket message must be response.create")
	}
	if len(event.Response) == 0 {
		return nil, nil, fmt.Errorf("response.create response is required")
	}

	var eventMap map[string]json.RawMessage
	if err := common.Unmarshal(message, &eventMap); err != nil {
		return nil, nil, err
	}

	request := &dto.OpenAIResponsesRequest{}
	if err := common.Unmarshal(event.Response, request); err != nil {
		return nil, nil, err
	}
	return eventMap, request, nil
}

func ensureResponsesWebSocketModelAccess(c *gin.Context, modelName string) *types.NewAPIError {
	if !common.GetContextKeyBool(c, constant.ContextKeyTokenModelLimitEnabled) {
		return nil
	}
	value, ok := common.GetContextKey(c, constant.ContextKeyTokenModelLimit)
	if !ok {
		return types.NewErrorWithStatusCode(errors.New("token has no model access"), types.ErrorCodeAccessDenied, http.StatusForbidden, types.ErrOptionWithSkipRetry())
	}
	tokenModelLimit, ok := value.(map[string]bool)
	if !ok {
		tokenModelLimit = map[string]bool{}
	}
	matchName := ratio_setting.FormatMatchingModelName(modelName)
	if !tokenModelLimit[matchName] {
		return types.NewErrorWithStatusCode(fmt.Errorf("token has no access to model %s", modelName), types.ErrorCodeAccessDenied, http.StatusForbidden, types.ErrOptionWithSkipRetry())
	}
	return nil
}

func selectResponsesWebSocketChannel(c *gin.Context, modelName string) (*model.Channel, *types.NewAPIError) {
	if channelIdValue, ok := common.GetContextKey(c, constant.ContextKeyTokenSpecificChannelId); ok {
		channelId, err := parseSpecificChannelID(channelIdValue)
		if err != nil {
			return nil, types.NewErrorWithStatusCode(err, types.ErrorCodeGetChannelFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
		}
		channel, err := model.GetChannelById(channelId, true)
		if err != nil {
			return nil, types.NewErrorWithStatusCode(err, types.ErrorCodeGetChannelFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
		}
		if channel.Status != common.ChannelStatusEnabled {
			return nil, types.NewErrorWithStatusCode(errors.New("channel is disabled"), types.ErrorCodeGetChannelFailed, http.StatusForbidden, types.ErrOptionWithSkipRetry())
		}
		if !isResponsesWebSocketChannelType(channel.Type) {
			return nil, types.NewErrorWithStatusCode(fmt.Errorf("channel type must support responses websocket"), types.ErrorCodeGetChannelFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
		}
		return channel, nil
	}

	tokenGroup := common.GetContextKeyString(c, constant.ContextKeyTokenGroup)
	if tokenGroup == "" {
		tokenGroup = common.GetContextKeyString(c, constant.ContextKeyUserGroup)
	}

	var (
		channel     *model.Channel
		selectedGrp = tokenGroup
		err         error
	)
	if tokenGroup == "auto" {
		userGroup := common.GetContextKeyString(c, constant.ContextKeyUserGroup)
		for _, group := range service.GetUserAutoGroup(userGroup) {
			channel, err = model.GetRandomSatisfiedChannel(group, modelName, 0, responsesWebSocketChannelTypes...)
			if err != nil {
				return nil, types.NewError(err, types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
			}
			if channel != nil {
				selectedGrp = group
				common.SetContextKey(c, constant.ContextKeyAutoGroup, group)
				break
			}
		}
	} else {
		channel, err = model.GetRandomSatisfiedChannel(tokenGroup, modelName, 0, responsesWebSocketChannelTypes...)
		if err != nil {
			return nil, types.NewError(err, types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
		}
	}
	if channel == nil {
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("no %s channel found, group: %s, model: %s", constant.GetChannelTypeName(constant.ChannelTypeResponsesWS), selectedGrp, modelName),
			types.ErrorCodeModelNotFound,
			http.StatusServiceUnavailable,
			types.ErrOptionWithSkipRetry(),
		)
	}
	return channel, nil
}

func isResponsesWebSocketChannelType(channelType int) bool {
	for _, supportedType := range responsesWebSocketChannelTypes {
		if channelType == supportedType {
			return true
		}
	}
	return false
}

func parseSpecificChannelID(value any) (int, error) {
	switch v := value.(type) {
	case string:
		id, err := strconv.Atoi(v)
		if err != nil {
			return 0, err
		}
		return id, nil
	case int:
		return v, nil
	default:
		return 0, fmt.Errorf("invalid specific channel id")
	}
}

func prepareResponsesWebSocketCreateMessage(
	c *gin.Context,
	info *relaycommon.RelayInfo,
	event map[string]json.RawMessage,
	request *dto.OpenAIResponsesRequest,
) ([]byte, *types.NewAPIError) {
	if err := helper.ModelMappedHelper(c, info, request); err != nil {
		return nil, types.NewError(err, types.ErrorCodeChannelModelMappedError, types.ErrOptionWithSkipRetry())
	}

	adaptor := relay.GetAdaptor(info.ApiType)
	if adaptor == nil {
		return nil, types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	adaptor.Init(info)

	converted, err := adaptor.ConvertOpenAIResponsesRequest(c, info, *request)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	convertedRequest, ok := converted.(dto.OpenAIResponsesRequest)
	if !ok {
		return nil, types.NewError(fmt.Errorf("invalid converted request type %T", converted), types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}

	responseBytes, newAPIError := patchResponsesWebSocketResponse(event["response"], convertedRequest, info)
	if newAPIError != nil {
		return nil, newAPIError
	}
	event["response"] = responseBytes

	message, err := common.Marshal(event)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	relaycommon.AppendRequestConversionFromRequest(info, convertedRequest)
	return message, nil
}

func patchResponsesWebSocketResponse(raw json.RawMessage, request dto.OpenAIResponsesRequest, info *relaycommon.RelayInfo) ([]byte, *types.NewAPIError) {
	var response map[string]json.RawMessage
	if err := common.Unmarshal(raw, &response); err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}

	modelBytes, err := common.Marshal(request.Model)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	response["model"] = modelBytes

	if request.Reasoning != nil {
		reasoningBytes, err := common.Marshal(request.Reasoning)
		if err != nil {
			return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}
		response["reasoning"] = reasoningBytes
	}

	responseBytes, err := common.Marshal(response)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	responseBytes, err = relaycommon.RemoveDisabledFields(responseBytes, info.ChannelOtherSettings, info.ChannelSetting.PassThroughBodyEnabled)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	if len(info.ParamOverride) > 0 {
		responseBytes, err = relaycommon.ApplyParamOverrideWithRelayInfo(responseBytes, info)
		if err != nil {
			if fixedErr, ok := relaycommon.AsParamOverrideReturnError(err); ok {
				return nil, relaycommon.NewAPIErrorFromParamOverride(fixedErr)
			}
			return nil, types.NewError(err, types.ErrorCodeChannelParamOverrideInvalid, types.ErrOptionWithSkipRetry())
		}
	}
	return responseBytes, nil
}

func preConsumeResponsesWebSocketRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.OpenAIResponsesRequest) *types.NewAPIError {
	now := time.Now()
	info.StartTime = now
	info.FirstResponseTime = now.Add(-time.Second)
	info.Billing = nil
	resetResponsesWebSocketUsageInfo(info, request)
	meta := request.GetTokenCountMeta()
	if setting.ShouldCheckPromptSensitive() && meta != nil {
		contains, words := service.CheckSensitiveText(meta.CombineText)
		if contains {
			logger.LogWarn(c, fmt.Sprintf("user sensitive words detected: %s", strings.Join(words, ", ")))
			return types.NewError(errors.New("sensitive words detected"), types.ErrorCodeSensitiveWordsDetected, types.ErrOptionWithSkipRetry())
		}
	}

	tokens, err := service.EstimateRequestToken(c, meta, info)
	if err != nil {
		return types.NewError(err, types.ErrorCodeCountTokenFailed)
	}
	info.SetEstimatePromptTokens(tokens)

	priceData, err := helper.ModelPriceHelper(c, info, tokens, meta)
	if err != nil {
		return types.NewError(err, types.ErrorCodeModelPriceError, types.ErrOptionWithStatusCode(http.StatusBadRequest))
	}
	if priceData.FreeModel {
		return nil
	}
	return service.PreConsumeBilling(c, priceData.QuotaToPreConsume, info)
}

func resetResponsesWebSocketUsageInfo(info *relaycommon.RelayInfo, request *dto.OpenAIResponsesRequest) {
	info.ResponsesUsageInfo = &relaycommon.ResponsesUsageInfo{
		BuiltInTools: make(map[string]*relaycommon.BuildInToolInfo),
	}
	if request == nil || len(request.Tools) == 0 {
		return
	}
	for _, tool := range request.GetToolsMap() {
		toolType := common.Interface2String(tool["type"])
		info.ResponsesUsageInfo.BuiltInTools[toolType] = &relaycommon.BuildInToolInfo{
			ToolName: toolType,
		}
		if toolType == dto.BuildInToolWebSearchPreview {
			searchContextSize := common.Interface2String(tool["search_context_size"])
			if searchContextSize == "" {
				searchContextSize = "medium"
			}
			info.ResponsesUsageInfo.BuiltInTools[toolType].SearchContextSize = searchContextSize
		}
	}
}

func dialResponsesWebSocket(c *gin.Context, info *relaycommon.RelayInfo) (*websocket.Conn, error) {
	adaptor := relay.GetAdaptor(info.ApiType)
	if adaptor == nil {
		return nil, fmt.Errorf("invalid api type: %d", info.ApiType)
	}
	adaptor.Init(info)
	resp, err := adaptor.DoRequest(c, info, nil)
	if err != nil {
		return nil, err
	}
	conn, ok := resp.(*websocket.Conn)
	if !ok || conn == nil {
		return nil, fmt.Errorf("invalid websocket upstream response")
	}
	return conn, nil
}

func forwardResponsesWebSocket(c *gin.Context, info *relaycommon.RelayInfo, state *responsesWebSocketState) {
	errCh := make(chan error, 2)
	go forwardResponsesWebSocketClient(c, info, state, errCh)
	go forwardResponsesWebSocketUpstream(c, info, state, errCh)
	err := <-errCh
	if err != nil && !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
		logger.LogError(c, "responses websocket relay closed: "+err.Error())
	}
	state.refund(c)
}

func forwardResponsesWebSocketClient(c *gin.Context, info *relaycommon.RelayInfo, state *responsesWebSocketState, errCh chan<- error) {
	for {
		messageType, message, err := info.ClientWs.ReadMessage()
		if err != nil {
			errCh <- err
			return
		}
		if messageType == websocket.TextMessage && isResponsesWebSocketCreateMessage(message) {
			upstreamMessage, newAPIError := handleResponsesWebSocketCreateMessage(c, info, state, message)
			if newAPIError != nil {
				state.writeClientError(c, info.ClientWs, newAPIError.ToOpenAIError())
				continue
			}
			message = upstreamMessage
		}
		if err := info.TargetWs.WriteMessage(messageType, message); err != nil {
			errCh <- err
			return
		}
	}
}

func forwardResponsesWebSocketUpstream(c *gin.Context, info *relaycommon.RelayInfo, state *responsesWebSocketState, errCh chan<- error) {
	for {
		messageType, message, err := info.TargetWs.ReadMessage()
		if err != nil {
			errCh <- err
			return
		}
		if err := state.writeClientMessage(info.ClientWs, messageType, message); err != nil {
			errCh <- err
			return
		}
		if messageType == websocket.TextMessage {
			handleResponsesWebSocketUpstreamEvent(c, info, state, message)
		}
	}
}

func isResponsesWebSocketCreateMessage(message []byte) bool {
	var event responsesWebSocketCreateEvent
	return common.Unmarshal(message, &event) == nil && event.Type == "response.create"
}

func handleResponsesWebSocketCreateMessage(c *gin.Context, info *relaycommon.RelayInfo, state *responsesWebSocketState, message []byte) ([]byte, *types.NewAPIError) {
	if !state.tryBegin() {
		return nil, types.NewErrorWithStatusCode(errors.New("only one in-flight response is supported per websocket connection"), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	event, request, err := parseResponsesWebSocketCreateEvent(message)
	if err != nil {
		state.finish()
		return nil, types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	if request.Model != "" && request.Model != info.OriginModelName {
		state.finish()
		return nil, types.NewErrorWithStatusCode(errors.New("model cannot change within a responses websocket connection"), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	if request.Model == "" {
		request.Model = info.OriginModelName
	}

	upstreamMessage, newAPIError := prepareResponsesWebSocketCreateMessage(c, info, event, request)
	if newAPIError != nil {
		state.finish()
		return nil, newAPIError
	}
	if newAPIError = preConsumeResponsesWebSocketRequest(c, info, request); newAPIError != nil {
		state.finish()
		return nil, newAPIError
	}
	state.setBilling(info.Billing)
	return upstreamMessage, nil
}

func handleResponsesWebSocketUpstreamEvent(c *gin.Context, info *relaycommon.RelayInfo, state *responsesWebSocketState, message []byte) {
	var streamResponse dto.ResponsesStreamResponse
	if err := common.Unmarshal(message, &streamResponse); err != nil {
		return
	}

	switch streamResponse.Type {
	case "response.completed":
		usage := responsesWebSocketUsage(c, info, streamResponse)
		service.PostTextConsumeQuota(c, info, usage, nil)
		state.finish()
	case "response.failed", "response.cancelled", "response.incomplete", "error":
		state.refund(c)
	case dto.ResponsesOutputTypeItemDone:
		recordResponsesWebSocketToolCall(info, streamResponse)
	}
}

func recordResponsesWebSocketToolCall(info *relaycommon.RelayInfo, streamResponse dto.ResponsesStreamResponse) {
	if info == nil || info.ResponsesUsageInfo == nil || info.ResponsesUsageInfo.BuiltInTools == nil || streamResponse.Item == nil {
		return
	}
	var toolType string
	switch streamResponse.Item.Type {
	case dto.BuildInCallWebSearchCall:
		toolType = dto.BuildInToolWebSearchPreview
	case dto.BuildInCallFileSearchCall:
		toolType = dto.BuildInToolFileSearch
	default:
		return
	}
	toolInfo, exists := info.ResponsesUsageInfo.BuiltInTools[toolType]
	if !exists || toolInfo == nil {
		return
	}
	toolInfo.CallCount++
}

func responsesWebSocketUsage(c *gin.Context, info *relaycommon.RelayInfo, streamResponse dto.ResponsesStreamResponse) *dto.Usage {
	usage := &dto.Usage{}
	if streamResponse.Response != nil {
		if streamResponse.Response.Usage != nil {
			usage.PromptTokens = streamResponse.Response.Usage.InputTokens
			usage.CompletionTokens = streamResponse.Response.Usage.OutputTokens
			usage.TotalTokens = streamResponse.Response.Usage.TotalTokens
			if streamResponse.Response.Usage.InputTokensDetails != nil {
				usage.PromptTokensDetails.CachedTokens = streamResponse.Response.Usage.InputTokensDetails.CachedTokens
			}
		}
		if streamResponse.Response.HasImageGenerationCall() {
			c.Set("image_generation_call", true)
			c.Set("image_generation_call_quality", streamResponse.Response.GetQuality())
			c.Set("image_generation_call_size", streamResponse.Response.GetSize())
		}
	}
	if usage.PromptTokens == 0 && usage.CompletionTokens != 0 {
		usage.PromptTokens = info.GetEstimatePromptTokens()
	}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}
	return usage
}

func writeResponsesWebSocketError(c *gin.Context, conn *websocket.Conn, openaiError types.OpenAIError) {
	if conn == nil {
		return
	}
	if err := helper.WssObject(c, conn, gin.H{
		"type":  "error",
		"error": openaiError,
	}); err != nil {
		logger.LogError(c, "failed to write responses websocket error: "+err.Error())
	}
}

func (s *responsesWebSocketState) begin(billing relaycommon.BillingSettler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inFlight = true
	s.billing = billing
}

func (s *responsesWebSocketState) tryBegin() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.inFlight {
		return false
	}
	s.inFlight = true
	return true
}

func (s *responsesWebSocketState) setBilling(billing relaycommon.BillingSettler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.billing = billing
}

func (s *responsesWebSocketState) finish() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inFlight = false
	s.billing = nil
}

func (s *responsesWebSocketState) refund(c *gin.Context) {
	s.mu.Lock()
	billing := s.billing
	s.inFlight = false
	s.billing = nil
	s.mu.Unlock()
	if billing != nil && billing.NeedsRefund() {
		billing.Refund(c)
	}
}

func (s *responsesWebSocketState) writeClientMessage(conn *websocket.Conn, messageType int, message []byte) error {
	s.clientWriteMu.Lock()
	defer s.clientWriteMu.Unlock()
	return conn.WriteMessage(messageType, message)
}

func (s *responsesWebSocketState) writeClientError(c *gin.Context, conn *websocket.Conn, openaiError types.OpenAIError) {
	s.clientWriteMu.Lock()
	defer s.clientWriteMu.Unlock()
	writeResponsesWebSocketError(c, conn, openaiError)
}

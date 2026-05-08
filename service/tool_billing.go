package service

import (
	"math"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

// ToolCallUsage captures all tool call counts from a single request.
type ToolCallUsage struct {
	ModelName              string
	WebSearchCalls         int
	WebSearchToolName      string // "web_search_preview", "web_search", etc.
	FileSearchCalls        int
	ImageGenerationCall    bool
	ImageGenerationQuality string
	ImageGenerationSize    string
}

// ToolCallItem represents a single billed tool usage line.
type ToolCallItem struct {
	Name       string  `json:"name"`
	CallCount  int     `json:"call_count"`
	PricePer1K float64 `json:"price_per_1k"`
	TotalPrice float64 `json:"total_price"`
	Quota      int     `json:"quota"`
}

// ToolCallResult holds the aggregated tool call billing for a request.
type ToolCallResult struct {
	TotalQuota int            `json:"total_quota"`
	Items      []ToolCallItem `json:"items,omitempty"`
}

// ComputeToolCallQuota calculates the total quota for all tool calls in a
// request. Tool prices are resolved via GetToolPriceForModel which supports
// model-prefix overrides. groupRatio is applied.
func ComputeToolCallQuota(usage ToolCallUsage, groupRatio float64) ToolCallResult {
	var items []ToolCallItem
	totalQuota := 0

	addItem := func(toolName string, count int) {
		if count <= 0 {
			return
		}
		pricePer1K := operation_setting.GetToolPriceForModel(toolName, usage.ModelName)
		if pricePer1K <= 0 {
			return
		}
		totalPrice := pricePer1K * float64(count) / 1000
		quota := int(math.Round(totalPrice * common.QuotaPerUnit * groupRatio))
		items = append(items, ToolCallItem{
			Name:       toolName,
			CallCount:  count,
			PricePer1K: pricePer1K,
			TotalPrice: totalPrice,
			Quota:      quota,
		})
		totalQuota += quota
	}

	if usage.WebSearchCalls > 0 && usage.WebSearchToolName != "" {
		addItem(usage.WebSearchToolName, usage.WebSearchCalls)
	}

	if usage.FileSearchCalls > 0 {
		addItem("file_search", usage.FileSearchCalls)
	}

	if usage.ImageGenerationCall {
		price := operation_setting.GetGPTImage1PriceOnceCall(usage.ImageGenerationQuality, usage.ImageGenerationSize)
		quota := int(math.Round(price * common.QuotaPerUnit * groupRatio))
		items = append(items, ToolCallItem{
			Name:       "image_generation",
			CallCount:  1,
			PricePer1K: price,
			TotalPrice: price,
			Quota:      quota,
		})
		totalQuota += quota
	}

	return ToolCallResult{
		TotalQuota: totalQuota,
		Items:      items,
	}
}

func ComputeResponsesImageGenerationToolPrice(usage *dto.ResponsesImageGenerationToolUsage) float64 {
	if usage == nil {
		return 0
	}

	modelRatio, ok, _ := ratio_setting.GetModelRatio(usage.Model)
	if !ok {
		return operation_setting.GetToolPriceForModel("image_generation", usage.Model) / 1000
	}

	inputTokens := usage.InputTokens
	outputTokens := usage.OutputTokens
	cachedTokens := 0
	imageInputTokens := 0
	if usage.InputTokensDetails != nil {
		cachedTokens = usage.InputTokensDetails.CachedTokens
		imageInputTokens = usage.InputTokensDetails.ImageTokens
	}
	if inputTokens < cachedTokens+imageInputTokens {
		inputTokens = cachedTokens + imageInputTokens
	}
	textInputTokens := inputTokens - cachedTokens - imageInputTokens

	inputPrice := modelRatio * 2
	completionRatio := ratio_setting.GetCompletionRatio(usage.Model)
	cacheRatio, _ := ratio_setting.GetCacheRatio(usage.Model)
	imageRatio, _ := ratio_setting.GetImageRatio(usage.Model)

	return (float64(textInputTokens)*inputPrice +
		float64(cachedTokens)*inputPrice*cacheRatio +
		float64(imageInputTokens)*inputPrice*imageRatio +
		float64(outputTokens)*inputPrice*completionRatio) / 1000000
}

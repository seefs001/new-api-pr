package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	common2 "github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/gin-gonic/gin"
)

const xaiResponsesToolAliasesContextKey = "xai_responses_tool_aliases"

type xaiResponsesToolAlias struct {
	Namespace string
	Name      string
}

func PrepareXAIResponsesRequest(c *gin.Context, info *RelayInfo, request dto.OpenAIResponsesRequest) (dto.OpenAIResponsesRequest, error) {
	if !isXAIResponsesCompatibilityChannel(info) {
		return request, nil
	}

	tools, aliases, err := normalizeXAIResponsesTools(request.Tools)
	if err != nil {
		return request, err
	}
	input, err := normalizeXAIResponsesInput(request.Input)
	if err != nil {
		return request, err
	}
	request.Tools = tools
	request.Input = input
	if c != nil {
		c.Set(xaiResponsesToolAliasesContextKey, aliases)
	}
	return request, nil
}

func NormalizeXAIResponsesResponse(c *gin.Context, info *RelayInfo, data []byte) ([]byte, error) {
	if !isXAIResponsesCompatibilityChannel(info) || c == nil || !bytes.Contains(data, []byte(`"function_call"`)) {
		return data, nil
	}
	value, exists := c.Get(xaiResponsesToolAliasesContextKey)
	if !exists {
		return data, nil
	}
	aliases, ok := value.(map[string]xaiResponsesToolAlias)
	if !ok || len(aliases) == 0 {
		return data, nil
	}

	var response any
	if err := common2.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("xAI responses compatibility: invalid response: %w", err)
	}
	if !restoreXAIResponsesToolNames(response, aliases) {
		return data, nil
	}
	return common2.Marshal(response)
}

func isXAIResponsesCompatibilityChannel(info *RelayInfo) bool {
	if info == nil || info.ChannelMeta == nil {
		return false
	}
	return info.ChannelType == constant.ChannelTypeXai || info.ChannelType == constant.ChannelTypeGrok
}

func normalizeXAIResponsesTools(raw json.RawMessage) (json.RawMessage, map[string]xaiResponsesToolAlias, error) {
	aliases := make(map[string]xaiResponsesToolAlias)
	if len(raw) == 0 {
		return raw, aliases, nil
	}

	var tools []map[string]json.RawMessage
	if err := common2.Unmarshal(raw, &tools); err != nil {
		return nil, nil, fmt.Errorf("xAI responses compatibility: invalid tools: %w", err)
	}
	normalized := make([]map[string]json.RawMessage, 0, len(tools)+1)
	functionNames := make(map[string]struct{})
	hasWebSearch := false
	hasXSearch := false

	for _, tool := range tools {
		toolType, err := rawJSONString(tool["type"])
		if err != nil {
			return nil, nil, fmt.Errorf("xAI responses compatibility: invalid tool type: %w", err)
		}
		delete(tool, "external_web_access")

		switch toolType {
		case "namespace":
			namespace, err := rawJSONString(tool["name"])
			if err != nil || strings.TrimSpace(namespace) == "" {
				return nil, nil, fmt.Errorf("xAI responses compatibility: namespace tool name is required")
			}
			var namespaceTools []map[string]json.RawMessage
			if err := common2.Unmarshal(tool["tools"], &namespaceTools); err != nil {
				return nil, nil, fmt.Errorf("xAI responses compatibility: invalid namespace %q tools: %w", namespace, err)
			}
			for _, namespaceTool := range namespaceTools {
				name, err := rawJSONString(namespaceTool["name"])
				if err != nil || strings.TrimSpace(name) == "" {
					return nil, nil, fmt.Errorf("xAI responses compatibility: function name in namespace %q is required", namespace)
				}
				flatName := strings.TrimRight(namespace, "_") + "__" + strings.TrimLeft(name, "_")
				if _, exists := functionNames[flatName]; exists {
					return nil, nil, fmt.Errorf("xAI responses compatibility: duplicate flattened function name %q", flatName)
				}
				functionNames[flatName] = struct{}{}
				aliases[flatName] = xaiResponsesToolAlias{Namespace: namespace, Name: name}
				delete(namespaceTool, "external_web_access")
				delete(namespaceTool, "strict")
				namespaceTool["type"] = json.RawMessage(`"function"`)
				encodedName, err := common2.Marshal(flatName)
				if err != nil {
					return nil, nil, err
				}
				namespaceTool["name"] = encodedName
				normalized = append(normalized, namespaceTool)
			}
		case "function":
			delete(tool, "strict")
			name, err := rawJSONString(tool["name"])
			if err != nil || strings.TrimSpace(name) == "" {
				return nil, nil, fmt.Errorf("xAI responses compatibility: function name is required")
			}
			if _, exists := functionNames[name]; exists {
				return nil, nil, fmt.Errorf("xAI responses compatibility: duplicate function name %q", name)
			}
			functionNames[name] = struct{}{}
			normalized = append(normalized, tool)
		case "web_search":
			hasWebSearch = true
			normalized = append(normalized, tool)
		case "x_search":
			hasXSearch = true
			normalized = append(normalized, tool)
		default:
			normalized = append(normalized, tool)
		}
	}
	if hasWebSearch && !hasXSearch {
		normalized = append(normalized, map[string]json.RawMessage{"type": json.RawMessage(`"x_search"`)})
	}

	encoded, err := common2.Marshal(normalized)
	if err != nil {
		return nil, nil, err
	}
	return encoded, aliases, nil
}

func normalizeXAIResponsesInput(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 || common2.GetJsonType(raw) != "array" {
		return raw, nil
	}

	var input []json.RawMessage
	if err := common2.Unmarshal(raw, &input); err != nil {
		return nil, fmt.Errorf("xAI responses compatibility: invalid input: %w", err)
	}
	normalized := make([]json.RawMessage, 0, len(input)+1)
	images := make([]map[string]json.RawMessage, 0)

	for _, rawItem := range input {
		if common2.GetJsonType(rawItem) != "object" {
			normalized = append(normalized, rawItem)
			continue
		}
		var item map[string]json.RawMessage
		if err := common2.Unmarshal(rawItem, &item); err != nil {
			return nil, fmt.Errorf("xAI responses compatibility: invalid input item: %w", err)
		}
		itemType, err := rawJSONString(item["type"])
		if err != nil {
			normalized = append(normalized, rawItem)
			continue
		}
		if itemType == "function_call" {
			namespace, _ := rawJSONString(item["namespace"])
			if namespace != "" {
				name, err := rawJSONString(item["name"])
				if err != nil || strings.TrimSpace(name) == "" {
					return nil, fmt.Errorf("xAI responses compatibility: function call name in namespace %q is required", namespace)
				}
				flatName := strings.TrimRight(namespace, "_") + "__" + strings.TrimLeft(name, "_")
				encodedName, err := common2.Marshal(flatName)
				if err != nil {
					return nil, err
				}
				item["name"] = encodedName
			}
			delete(item, "namespace")
			encodedItem, err := common2.Marshal(item)
			if err != nil {
				return nil, err
			}
			normalized = append(normalized, encodedItem)
			continue
		}
		if itemType != "function_call_output" || common2.GetJsonType(item["output"]) != "array" {
			normalized = append(normalized, rawItem)
			continue
		}

		var parts []map[string]json.RawMessage
		if err := common2.Unmarshal(item["output"], &parts); err != nil {
			return nil, fmt.Errorf("xAI responses compatibility: invalid function_call_output: %w", err)
		}
		var text strings.Builder
		hasNonTextPart := false
		for _, part := range parts {
			partType, err := rawJSONString(part["type"])
			if err != nil {
				hasNonTextPart = true
				continue
			}
			switch partType {
			case "input_text":
				partText, err := rawJSONString(part["text"])
				if err == nil {
					text.WriteString(partText)
				}
			case "input_image":
				hasNonTextPart = true
				image, ok := normalizeXAIInputImage(part)
				if ok {
					images = append(images, image)
				}
			default:
				hasNonTextPart = true
			}
		}
		output := text.String()
		if output == "" && hasNonTextPart {
			output = "(see attached image)"
		}
		encodedOutput, err := common2.Marshal(output)
		if err != nil {
			return nil, err
		}
		item["output"] = encodedOutput
		encodedItem, err := common2.Marshal(item)
		if err != nil {
			return nil, err
		}
		normalized = append(normalized, encodedItem)
	}

	if len(images) > 0 {
		content := make([]map[string]json.RawMessage, 0, len(images)+1)
		content = append(content, map[string]json.RawMessage{
			"type": json.RawMessage(`"input_text"`),
			"text": json.RawMessage(`"Attached image(s) from tool result:"`),
		})
		content = append(content, images...)
		message := struct {
			Type    string                       `json:"type"`
			Role    string                       `json:"role"`
			Content []map[string]json.RawMessage `json:"content"`
		}{Type: "message", Role: "user", Content: content}
		encodedMessage, err := common2.Marshal(message)
		if err != nil {
			return nil, err
		}
		normalized = append(normalized, encodedMessage)
	}

	encoded, err := common2.Marshal(normalized)
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

func normalizeXAIInputImage(part map[string]json.RawMessage) (map[string]json.RawMessage, bool) {
	imageURL, _ := rawJSONString(part["image_url"])
	if imageURL == "" {
		var source map[string]json.RawMessage
		if common2.Unmarshal(part["source"], &source) == nil {
			imageURL, _ = rawJSONString(source["url"])
			if imageURL == "" {
				data, _ := rawJSONString(source["data"])
				if data != "" {
					mediaType, _ := rawJSONString(source["media_type"])
					if mediaType == "" {
						mediaType = "image/png"
					}
					imageURL = "data:" + mediaType + ";base64," + data
				}
			}
		}
	}
	if imageURL == "" {
		return nil, false
	}

	encodedURL, err := common2.Marshal(imageURL)
	if err != nil {
		return nil, false
	}
	image := map[string]json.RawMessage{
		"type":      json.RawMessage(`"input_image"`),
		"image_url": encodedURL,
	}
	if detail, err := rawJSONString(part["detail"]); err == nil && detail != "" {
		encodedDetail, err := common2.Marshal(detail)
		if err == nil {
			image["detail"] = encodedDetail
		}
	}
	return image, true
}

func rawJSONString(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", fmt.Errorf("missing string")
	}
	var value string
	if err := common2.Unmarshal(raw, &value); err != nil {
		return "", err
	}
	return value, nil
}

func restoreXAIResponsesToolNames(node any, aliases map[string]xaiResponsesToolAlias) bool {
	changed := false
	switch value := node.(type) {
	case []any:
		for _, item := range value {
			changed = restoreXAIResponsesToolNames(item, aliases) || changed
		}
	case map[string]any:
		if value["type"] == "function_call" {
			if name, ok := value["name"].(string); ok {
				if alias, exists := aliases[name]; exists {
					value["namespace"] = alias.Namespace
					value["name"] = alias.Name
					changed = true
				}
			}
		}
		for _, child := range value {
			changed = restoreXAIResponsesToolNames(child, aliases) || changed
		}
	}
	return changed
}

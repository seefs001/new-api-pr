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
	Custom    bool
}

type xaiResponsesCompatibilityState struct {
	Aliases       map[string]xaiResponsesToolAlias
	CustomCallIDs map[string]struct{}
	CustomItemIDs map[string]struct{}
}

func PrepareXAIResponsesRequest(c *gin.Context, info *RelayInfo, request dto.OpenAIResponsesRequest) (dto.OpenAIResponsesRequest, error) {
	if !isXAIResponsesCompatibilityChannel(info) {
		return request, nil
	}

	tools, aliases, err := normalizeXAIResponsesTools(request.Tools)
	if err != nil {
		return request, err
	}
	toolChoice, err := normalizeXAIResponsesToolChoice(request.ToolChoice)
	if err != nil {
		return request, err
	}
	input, err := normalizeXAIResponsesInput(request.Input)
	if err != nil {
		return request, err
	}
	request.Tools = tools
	request.ToolChoice = toolChoice
	request.Input = input
	var normalizedTools []json.RawMessage
	if len(tools) == 0 || common2.Unmarshal(tools, &normalizedTools) != nil || len(normalizedTools) == 0 {
		request.Tools = nil
		request.ToolChoice = nil
		request.ParallelToolCalls = nil
	}
	if c != nil {
		c.Set(xaiResponsesToolAliasesContextKey, &xaiResponsesCompatibilityState{
			Aliases:       aliases,
			CustomCallIDs: make(map[string]struct{}),
			CustomItemIDs: make(map[string]struct{}),
		})
	}
	return request, nil
}

func NormalizeXAIResponsesResponse(c *gin.Context, info *RelayInfo, data []byte) ([]byte, error) {
	if !isXAIResponsesCompatibilityChannel(info) || c == nil || !bytes.Contains(data, []byte("function_call")) {
		return data, nil
	}
	value, exists := c.Get(xaiResponsesToolAliasesContextKey)
	if !exists {
		return data, nil
	}
	state, ok := value.(*xaiResponsesCompatibilityState)
	if !ok || len(state.Aliases) == 0 {
		return data, nil
	}

	var response any
	if err := common2.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("xAI responses compatibility: invalid response: %w", err)
	}
	if !restoreXAIResponsesToolNames(response, state) {
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
		case "custom":
			name, err := rawJSONString(tool["name"])
			if err != nil || strings.TrimSpace(name) == "" {
				return nil, nil, fmt.Errorf("xAI responses compatibility: custom tool name is required")
			}
			if _, exists := functionNames[name]; exists {
				return nil, nil, fmt.Errorf("xAI responses compatibility: duplicate function name %q", name)
			}
			functionNames[name] = struct{}{}
			aliases[name] = xaiResponsesToolAlias{Name: name, Custom: true}
			description, _ := rawJSONString(tool["description"])
			const freeformInstruction = "This is a FREEFORM tool, so do not wrap the patch in JSON."
			if strings.Contains(description, freeformInstruction) {
				description = strings.Replace(description, freeformInstruction, "Pass the complete freeform input in the input field.", 1)
			} else {
				description = strings.TrimSpace(description) + " Pass the complete freeform input in the input field."
			}
			encodedDescription, err := common2.Marshal(strings.TrimSpace(description))
			if err != nil {
				return nil, nil, err
			}
			tool["type"] = json.RawMessage(`"function"`)
			tool["description"] = encodedDescription
			tool["parameters"] = json.RawMessage(`{"type":"object","properties":{"input":{"type":"string","description":"Complete raw input for this tool."}},"required":["input"],"additionalProperties":false}`)
			delete(tool, "format")
			delete(tool, "strict")
			normalized = append(normalized, tool)
		case "web_search":
			hasWebSearch = true
			normalized = append(normalized, tool)
		case "x_search":
			hasXSearch = true
			normalized = append(normalized, tool)
		case "image_generation", "collections_search", "file_search", "code_execution", "code_interpreter", "mcp", "shell":
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

func normalizeXAIResponsesToolChoice(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 || common2.GetJsonType(raw) != "object" {
		return raw, nil
	}

	var choice map[string]json.RawMessage
	if err := common2.Unmarshal(raw, &choice); err != nil {
		return nil, fmt.Errorf("xAI responses compatibility: invalid tool_choice: %w", err)
	}
	choiceType, _ := rawJSONString(choice["type"])
	if choiceType == "allowed_tools" && common2.GetJsonType(choice["tools"]) == "array" {
		var tools []map[string]json.RawMessage
		if err := common2.Unmarshal(choice["tools"], &tools); err != nil {
			return nil, fmt.Errorf("xAI responses compatibility: invalid allowed tools: %w", err)
		}
		for _, tool := range tools {
			if err := normalizeXAIResponsesToolChoiceEntry(tool); err != nil {
				return nil, err
			}
		}
		encodedTools, err := common2.Marshal(tools)
		if err != nil {
			return nil, err
		}
		choice["tools"] = encodedTools
	} else if err := normalizeXAIResponsesToolChoiceEntry(choice); err != nil {
		return nil, err
	}
	return common2.Marshal(choice)
}

func normalizeXAIResponsesToolChoiceEntry(choice map[string]json.RawMessage) error {
	choiceType, _ := rawJSONString(choice["type"])
	if choiceType == "custom" {
		choice["type"] = json.RawMessage(`"function"`)
		choiceType = "function"
	}
	if choiceType != "function" {
		return nil
	}

	namespace, _ := rawJSONString(choice["namespace"])
	if namespace == "" {
		return nil
	}
	name, err := rawJSONString(choice["name"])
	if err != nil || strings.TrimSpace(name) == "" {
		return fmt.Errorf("xAI responses compatibility: function tool choice name in namespace %q is required", namespace)
	}
	flatName := strings.TrimRight(namespace, "_") + "__" + strings.TrimLeft(name, "_")
	encodedName, err := common2.Marshal(flatName)
	if err != nil {
		return err
	}
	choice["name"] = encodedName
	delete(choice, "namespace")
	return nil
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
		if itemType == "custom_tool_call" {
			callID, callIDErr := rawJSONString(item["call_id"])
			name, nameErr := rawJSONString(item["name"])
			input, inputErr := rawJSONString(item["input"])
			if callIDErr != nil || strings.TrimSpace(callID) == "" || nameErr != nil || strings.TrimSpace(name) == "" || inputErr != nil {
				return nil, fmt.Errorf("xAI responses compatibility: invalid custom tool call")
			}
			arguments, err := common2.Marshal(struct {
				Input string `json:"input"`
			}{Input: input})
			if err != nil {
				return nil, err
			}
			encodedArguments, err := common2.Marshal(string(arguments))
			if err != nil {
				return nil, err
			}
			encodedCallID, err := common2.Marshal(callID)
			if err != nil {
				return nil, err
			}
			encodedName, err := common2.Marshal(name)
			if err != nil {
				return nil, err
			}
			encodedItem, err := common2.Marshal(map[string]json.RawMessage{
				"type":      json.RawMessage(`"function_call"`),
				"call_id":   encodedCallID,
				"name":      encodedName,
				"arguments": encodedArguments,
			})
			if err != nil {
				return nil, err
			}
			normalized = append(normalized, encodedItem)
			continue
		}
		if itemType == "custom_tool_call_output" {
			callID, err := rawJSONString(item["call_id"])
			if err != nil || strings.TrimSpace(callID) == "" {
				return nil, fmt.Errorf("xAI responses compatibility: invalid custom tool call output")
			}
			item["type"] = json.RawMessage(`"function_call_output"`)
			delete(item, "id")
			delete(item, "name")
			delete(item, "internal_chat_message_metadata_passthrough")
			itemType = "function_call_output"
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
			if itemType == "function_call_output" {
				encodedItem, err := common2.Marshal(item)
				if err != nil {
					return nil, err
				}
				normalized = append(normalized, encodedItem)
			} else {
				normalized = append(normalized, rawItem)
			}
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

func restoreXAIResponsesToolNames(node any, state *xaiResponsesCompatibilityState) bool {
	changed := false
	switch value := node.(type) {
	case []any:
		for _, item := range value {
			changed = restoreXAIResponsesToolNames(item, state) || changed
		}
	case map[string]any:
		for _, child := range value {
			changed = restoreXAIResponsesToolNames(child, state) || changed
		}
		if value["type"] == "function_call" {
			if name, ok := value["name"].(string); ok {
				if alias, exists := state.Aliases[name]; exists {
					if alias.Custom {
						arguments, _ := value["arguments"].(string)
						value["type"] = "custom_tool_call"
						value["name"] = alias.Name
						value["input"] = xaiResponsesCustomToolInput(arguments)
						delete(value, "arguments")
						delete(value, "namespace")
						if callID, ok := value["call_id"].(string); ok && callID != "" {
							state.CustomCallIDs[callID] = struct{}{}
						}
						if itemID, ok := value["id"].(string); ok && itemID != "" {
							state.CustomItemIDs[itemID] = struct{}{}
						}
					} else {
						value["namespace"] = alias.Namespace
						value["name"] = alias.Name
					}
					changed = true
				}
			}
		}
		eventType, _ := value["type"].(string)
		if eventType == "response.function_call_arguments.delta" || eventType == "response.function_call_arguments.done" {
			itemID, _ := value["item_id"].(string)
			callID, _ := value["call_id"].(string)
			_, customItem := state.CustomItemIDs[itemID]
			_, customCall := state.CustomCallIDs[callID]
			if customItem || customCall {
				if eventType == "response.function_call_arguments.delta" {
					value["type"] = "response.custom_tool_call_input.delta"
				} else {
					value["type"] = "response.custom_tool_call_input.done"
					arguments, _ := value["arguments"].(string)
					value["input"] = xaiResponsesCustomToolInput(arguments)
					delete(value, "arguments")
				}
				changed = true
			}
		}
	}
	return changed
}

func xaiResponsesCustomToolInput(arguments string) string {
	var object map[string]json.RawMessage
	if common2.Unmarshal([]byte(arguments), &object) == nil {
		if input, err := rawJSONString(object["input"]); err == nil {
			return input
		}
		if patch, err := rawJSONString(object["patch"]); err == nil {
			return patch
		}
	}
	return arguments
}

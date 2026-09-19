package relay

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

var pluginImageArtifactKeyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._~-]{0,127}$`)

// PluginImagesEnvelope owns the OpenAI Images wire envelope for a synchronous
// task-plugin observation. The plugin renders only the image list; the host
// validates it and adds the fields it owns.
type PluginImagesEnvelope struct {
	createdAt int64
	limits    PluginProtocolLimits
}

func NewPluginImagesEnvelope(createdAt int64, limits PluginProtocolLimits) *PluginImagesEnvelope {
	return &PluginImagesEnvelope{createdAt: createdAt, limits: limits.withDefaults()}
}

// FinalResponse validates a renderFinal payload of the openai_images protocol.
// The accepted plugin shape is {data: [{url?, b64_json?, artifact?, revised_prompt?}]};
// unknown fields are rejected so plugins cannot smuggle host-owned fields.
// An item naming an `artifact` key asks the host to inline that artifact's
// bytes as `b64_json`; inline fetches and encodes them.
func (e *PluginImagesEnvelope) FinalResponse(payload any, inline func(artifactKey string) (string, error)) (map[string]any, error) {
	encoded, err := common.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("images response is not JSON-compatible: %w", err)
	}
	if len(encoded) > e.limits.MaxTotalOutputBytes {
		return nil, fmt.Errorf("images response exceeds %d bytes", e.limits.MaxTotalOutputBytes)
	}
	depth, err := pluginJSONDepth(encoded)
	if err != nil || depth > e.limits.MaxEventDepth {
		return nil, fmt.Errorf("images response exceeds depth limit of %d", e.limits.MaxEventDepth)
	}
	var response map[string]any
	if err = common.Unmarshal(encoded, &response); err != nil || response == nil {
		return nil, errors.New("images response must be an object")
	}
	for name := range response {
		if name != "data" {
			return nil, fmt.Errorf("images response contains unsupported field %q", name)
		}
	}
	items, ok := response["data"].([]any)
	if !ok || len(items) == 0 {
		return nil, errors.New("images response data must be a non-empty array")
	}
	if len(items) > e.limits.MaxOutputs {
		return nil, fmt.Errorf("images response data exceeds limit of %d", e.limits.MaxOutputs)
	}
	data := make([]map[string]any, 0, len(items))
	for index, rawItem := range items {
		item, ok := rawItem.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("images response data[%d] must be an object", index)
		}
		image := make(map[string]any, len(item))
		artifactKey := ""
		for name, value := range item {
			text, isString := value.(string)
			switch name {
			case "artifact":
				if !isString || !pluginImageArtifactKeyPattern.MatchString(text) {
					return nil, fmt.Errorf("images response data[%d].artifact must be an artifact key", index)
				}
				artifactKey = text
				continue
			case "url":
				if !isString || !isAbsoluteHTTPURL(text) {
					return nil, fmt.Errorf("images response data[%d].url must be an absolute HTTP(S) URL", index)
				}
			case "b64_json":
				if !isString || text == "" {
					return nil, fmt.Errorf("images response data[%d].b64_json must be a non-empty string", index)
				}
			case "revised_prompt":
				if !isString {
					return nil, fmt.Errorf("images response data[%d].revised_prompt must be a string", index)
				}
			default:
				return nil, fmt.Errorf("images response data[%d] contains unsupported field %q", index, name)
			}
			image[name] = text
		}
		_, hasURL := image["url"]
		_, hasBase64 := image["b64_json"]
		switch {
		case artifactKey != "" && (hasURL || hasBase64):
			return nil, fmt.Errorf("images response data[%d] cannot combine artifact with url or b64_json", index)
		case artifactKey != "":
			if inline == nil {
				return nil, fmt.Errorf("images response data[%d] artifact inlining is unavailable", index)
			}
			encoded, err := inline(artifactKey)
			if err != nil {
				return nil, fmt.Errorf("images response data[%d] artifact %q: %w", index, artifactKey, err)
			}
			image["b64_json"] = encoded
		case !hasURL && !hasBase64:
			return nil, fmt.Errorf("images response data[%d] must contain url, b64_json or artifact", index)
		}
		data = append(data, image)
	}
	return map[string]any{"created": e.createdAt, "data": data}, nil
}

func isAbsoluteHTTPURL(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Host == "" {
		return false
	}
	return parsed.Scheme == "http" || parsed.Scheme == "https"
}

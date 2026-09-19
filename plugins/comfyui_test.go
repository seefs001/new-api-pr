package plugins_test

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	builtinplugins "github.com/QuantumNous/new-api/plugins"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func comfyuiCall(t *testing.T, plugin *jsplugin.LoadedPlugin, path []string, args ...any) map[string]any {
	t.Helper()
	var value any
	var err error
	if len(path) == 1 {
		value, err = plugin.Engine.Call(context.Background(), path[0], args...)
	} else {
		value, err = plugin.Engine.CallPath(context.Background(), path[0], path[1:], args...)
	}
	require.NoError(t, err)
	encoded, err := common.Marshal(value)
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, common.Unmarshal(encoded, &decoded))
	return decoded
}

func comfyuiHistory(statusStr string, completed bool, outputs map[string]any, messages []any) map[string]any {
	return map[string]any{
		"7f1c": map[string]any{
			"prompt":  []any{float64(3), "7f1c"},
			"outputs": outputs,
			"status":  map[string]any{"status_str": statusStr, "completed": completed, "messages": messages},
		},
	}
}

func TestComfyUIPluginContract(t *testing.T) {
	source, err := builtinplugins.Source("comfyui")
	require.NoError(t, err)
	registry := jsplugin.NewRegistry()
	plugin, err := registry.RegisterFactory(source, jsplugin.Options{Key: "comfyui"})
	require.NoError(t, err)
	generation := registry.Generation()

	t.Run("claims images and responses for every workflow model", func(t *testing.T) {
		require.NotEmpty(t, plugin.Meta.Models)
		for _, model := range plugin.Meta.Models {
			binding, found := generation.LookupEndpoint("POST", "/v1/images/generations", model)
			require.True(t, found, model)
			assert.Equal(t, "openai_images", binding.Protocol)
			binding, found = generation.LookupEndpoint("POST", "/v1/responses", model)
			require.True(t, found, model)
			assert.Equal(t, "openai_responses", binding.Protocol)
		}
		assert.Equal(t, "none", plugin.Meta.Auth.Type)
		assert.Equal(t, "http://127.0.0.1:8188", plugin.Meta.BaseURL)
	})

	decodeImages := func(body map[string]any) (map[string]any, error) {
		value, callErr := plugin.Engine.CallPath(context.Background(), "protocols", []string{"openai_images", "decodeRequest"}, map[string]any{
			"protocol": "openai_images", "operation": "create", "model": "comfyui-anima-aesthetic", "stream": false,
			"body": map[string]any{"kind": "json", "value": body},
		})
		if callErr != nil {
			return nil, callErr
		}
		encoded, marshalErr := common.Marshal(value)
		require.NoError(t, marshalErr)
		var decoded map[string]any
		require.NoError(t, common.Unmarshal(encoded, &decoded))
		return decoded, nil
	}

	var requestBody map[string]any
	t.Run("decodes the OpenAI Images request into workflow parameters", func(t *testing.T) {
		decoded, decodeErr := decodeImages(map[string]any{
			"model": "comfyui-anima-aesthetic", "prompt": " 1girl, garden ", "n": 2, "size": "1024x768",
			"steps": 20, "cfg": 5.5, "seed": 42, "negative_prompt": "lowres", "sampler_name": "euler", "quality": "hd", "response_format": "url",
		})
		require.NoError(t, decodeErr)
		assert.Equal(t, "submit", decoded["kind"])
		assert.Equal(t, "comfyui-anima-aesthetic", decoded["model"])
		assert.Equal(t, "text_to_image", decoded["action"])
		var ok bool
		requestBody, ok = decoded["requestBody"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, map[string]any{
			"model": "comfyui-anima-aesthetic", "prompt": "1girl, garden", "n": float64(2), "width": float64(1024), "height": float64(768),
			"steps": float64(20), "cfg": 5.5, "seed": float64(42), "negative_prompt": "lowres", "sampler_name": "euler",
		}, requestBody)

		defaults, decodeErr := decodeImages(map[string]any{"model": "comfyui-anima-aesthetic", "prompt": "a cat"})
		require.NoError(t, decodeErr)
		assert.Equal(t, map[string]any{"model": "comfyui-anima-aesthetic", "prompt": "a cat", "n": float64(1), "width": float64(1024), "height": float64(1024)}, defaults["requestBody"])
	})

	t.Run("rejects unbounded or unsupported image parameters", func(t *testing.T) {
		rejected := []struct {
			body map[string]any
			err  string
		}{
			{map[string]any{"model": "comfyui-anima-aesthetic"}, "prompt is required"},
			{map[string]any{"model": "comfyui-anima-aesthetic", "prompt": "x", "n": 5}, "n must be an integer between 1 and 4"},
			{map[string]any{"model": "comfyui-anima-aesthetic", "prompt": "x", "size": "1000x1000"}, "multiples of 16"},
			{map[string]any{"model": "comfyui-anima-aesthetic", "prompt": "x", "size": "large"}, "size must be WIDTHxHEIGHT"},
			{map[string]any{"model": "comfyui-anima-aesthetic", "prompt": "x", "response_format": "png"}, "response_format must be url or b64_json"},
			{map[string]any{"model": "comfyui-anima-aesthetic", "prompt": "x", "steps": 500}, "steps must be an integer between 1 and 150"},
			{map[string]any{"model": "comfyui-anima-aesthetic", "prompt": "x", "sampler_name": "euler; drop"}, "sampler_name must be a ComfyUI option name"},
		}
		for _, testCase := range rejected {
			_, decodeErr := decodeImages(testCase.body)
			require.ErrorContains(t, decodeErr, testCase.err)
		}
	})

	t.Run("decodes OpenAI Images edits from multipart and JSON bodies", func(t *testing.T) {
		decodeEdit := func(body map[string]any) (map[string]any, error) {
			value, callErr := plugin.Engine.CallPath(context.Background(), "protocols", []string{"openai_images", "decodeRequest"}, map[string]any{
				"protocol": "openai_images", "operation": "edit", "model": "comfyui-anima-aesthetic", "stream": false, "body": body,
			})
			if callErr != nil {
				return nil, callErr
			}
			encoded, marshalErr := common.Marshal(value)
			require.NoError(t, marshalErr)
			var decoded map[string]any
			require.NoError(t, common.Unmarshal(encoded, &decoded))
			return decoded, nil
		}
		multipart := map[string]any{
			"kind":   "multipart",
			"fields": map[string]any{"model": []string{"comfyui-anima-aesthetic"}, "prompt": []string{"make it snowy"}, "n": []string{"2"}, "strength": []string{"0.6"}, "steps": []string{"12"}},
			"files": []map[string]any{
				{"ref": "request_file:image[]", "field": "image[]", "filename": "src.png", "mimeType": "image/png", "size": 1024},
				{"ref": "request_file:mask", "field": "mask", "filename": "mask.png", "mimeType": "image/png", "size": 512},
			},
		}
		decoded, decodeErr := decodeEdit(multipart)
		require.NoError(t, decodeErr)
		assert.Equal(t, "inpaint", decoded["action"])
		assert.Equal(t, map[string]any{
			"model": "comfyui-anima-aesthetic", "prompt": "make it snowy", "n": float64(2), "steps": float64(12), "strength": 0.6,
			"image": map[string]any{"ref": "request_file:image[]"}, "mask": map[string]any{"ref": "request_file:mask"},
		}, decoded["requestBody"], "edits keep the source size unless size is given")

		jsonBody := map[string]any{"kind": "json", "value": map[string]any{
			"model": "comfyui-anima-aesthetic", "prompt": "make it snowy", "size": "512x768", "image": "data:image/png;base64,aGVs bG8=",
		}}
		decoded, decodeErr = decodeEdit(jsonBody)
		require.NoError(t, decodeErr)
		assert.Equal(t, "image_to_image", decoded["action"])
		requestBody := decoded["requestBody"].(map[string]any)
		assert.Equal(t, map[string]any{"base64": "aGVsbG8="}, requestBody["image"], "data URL prefix and whitespace are stripped for the loader node")
		assert.Equal(t, float64(512), requestBody["width"])
		assert.Equal(t, 0.75, requestBody["strength"])
		assert.Nil(t, requestBody["mask"])

		rejected := []struct {
			body map[string]any
			err  string
		}{
			{map[string]any{"kind": "multipart", "fields": map[string]any{"prompt": []string{"x"}}, "files": []map[string]any{}}, "image file is required"},
			{map[string]any{"kind": "multipart", "fields": map[string]any{"prompt": []string{"x"}, "strength": []string{"1.5"}}, "files": []map[string]any{{"ref": "request_file:image", "field": "image", "filename": "a.png", "mimeType": "image/png", "size": 1}}}, "strength must be a number greater than 0 and at most 1"},
			{map[string]any{"kind": "multipart", "fields": map[string]any{"prompt": []string{"x"}}, "files": []map[string]any{{"ref": "request_file:image", "field": "image", "filename": "a.gif", "mimeType": "image/gif", "size": 1}}}, "PNG, JPEG or WebP"},
			{map[string]any{"kind": "json", "value": map[string]any{"prompt": "x", "image": "https://cdn.example/a.png"}}, "URLs are not supported"},
		}
		for _, testCase := range rejected {
			_, decodeErr := decodeEdit(testCase.body)
			require.ErrorContains(t, decodeErr, testCase.err)
		}
	})

	t.Run("builds image-to-image and inpaint workflows with file placeholders", func(t *testing.T) {
		build := func(requestBody map[string]any, action string) map[string]any {
			descriptor := comfyuiCall(t, plugin, []string{"buildSubmitRequest"}, map[string]any{
				"requestBody": requestBody, "model": "comfyui-anima-aesthetic", "upstreamModel": "comfyui-anima-aesthetic", "action": action,
				"baseUrl": "http://comfy.local:8188", "publicTaskId": "task_edit",
			})
			return descriptor["body"].(map[string]any)["prompt"].(map[string]any)
		}
		node := func(workflow map[string]any, id string) map[string]any {
			return workflow[id].(map[string]any)["inputs"].(map[string]any)
		}

		img2img := build(map[string]any{"model": "comfyui-anima-aesthetic", "prompt": "snowy", "n": 1, "strength": 0.6, "image": map[string]any{"ref": "request_file:image"}}, "image_to_image")
		assert.Equal(t, "ETN_LoadImageBase64", img2img["10"].(map[string]any)["class_type"])
		assert.Equal(t, map[string]any{"__fileRef": "request_file:image", "encoding": "base64", "maxBytes": float64(25 << 20)}, node(img2img, "10")["image"])
		assert.Equal(t, "VAEEncode", img2img["6"].(map[string]any)["class_type"])
		assert.Equal(t, []any{"10", float64(0)}, node(img2img, "6")["pixels"], "no ImageScale without an explicit size")
		assert.Equal(t, []any{"6", float64(0)}, node(img2img, "7")["latent_image"])
		assert.Equal(t, 0.6, node(img2img, "7")["denoise"])
		for _, absent := range []string{"11", "12", "17", "18"} {
			assert.NotContains(t, img2img, absent)
		}

		inpaint := build(map[string]any{
			"model": "comfyui-anima-aesthetic", "prompt": "snowy", "n": 2, "width": 512, "height": 768, "strength": 1,
			"image": map[string]any{"base64": "aGVsbG8="}, "mask": map[string]any{"ref": "request_file:mask"},
		}, "inpaint")
		assert.Equal(t, "aGVsbG8=", node(inpaint, "10")["image"], "inline Base64 is passed through verbatim")
		assert.Equal(t, map[string]any{"image": []any{"10", float64(0)}, "upscale_method": "lanczos", "width": float64(512), "height": float64(768), "crop": "center"}, node(inpaint, "11"))
		assert.Equal(t, []any{"11", float64(0)}, node(inpaint, "6")["pixels"])
		assert.Equal(t, []any{"12", float64(1)}, node(inpaint, "13")["mask"], "the mask comes from the alpha channel and is inverted")
		assert.Equal(t, "InvertMask", inpaint["13"].(map[string]any)["class_type"])
		assert.Equal(t, []any{"16", float64(0)}, node(inpaint, "17")["mask"], "a resized mask goes through MaskToImage, ImageScale and ImageToMask")
		assert.Equal(t, "SetLatentNoiseMask", inpaint["17"].(map[string]any)["class_type"])
		assert.Equal(t, map[string]any{"samples": []any{"17", float64(0)}, "amount": float64(2)}, node(inpaint, "18"))
		assert.Equal(t, []any{"18", float64(0)}, node(inpaint, "7")["latent_image"])
		assert.Equal(t, float64(1), node(inpaint, "7")["denoise"])
	})

	t.Run("decodes Responses input with the same normalization", func(t *testing.T) {
		decoded := comfyuiCall(t, plugin, []string{"protocols", "openai_responses", "decodeRequest"}, map[string]any{
			"model": "alias", "upstreamModel": "comfyui-anima-turbo", "stream": false,
			"body": map[string]any{"kind": "json", "value": map[string]any{
				"model": "alias", "size": "512x512",
				"input": []any{map[string]any{"role": "user", "content": []any{map[string]any{"type": "input_text", "text": "a red fox"}}}},
			}},
		})
		assert.Equal(t, "alias", decoded["model"])
		assert.Equal(t, map[string]any{"model": "alias", "prompt": "a red fox", "n": float64(1), "width": float64(512), "height": float64(512)}, decoded["requestBody"])
	})

	driverContext := map[string]any{
		"requestBody": requestBody, "model": "comfyui-anima-aesthetic", "upstreamModel": "comfyui-anima-aesthetic",
		"baseUrl": "http://comfy.local:8188", "apiKey": "placeholder", "publicTaskId": "task_abc", "action": "text_to_image",
	}
	t.Run("builds a ComfyUI prompt from the workflow template", func(t *testing.T) {
		descriptor := comfyuiCall(t, plugin, []string{"buildSubmitRequest"}, driverContext)
		assert.Equal(t, "http://comfy.local:8188/prompt", descriptor["url"])
		assert.Equal(t, "POST", descriptor["method"])
		assert.Equal(t, map[string]any{"Content-Type": "application/json", "Authorization": "Bearer placeholder"}, descriptor["headers"])
		body := descriptor["body"].(map[string]any)
		assert.NotEmpty(t, body["client_id"])
		workflow := body["prompt"].(map[string]any)
		node := func(id string) map[string]any { return workflow[id].(map[string]any)["inputs"].(map[string]any) }
		assert.Equal(t, "anima-aesthetic-v1.1.safetensors", node("1")["unet_name"])
		assert.Equal(t, "1girl, garden", node("4")["text"])
		assert.Equal(t, "lowres", node("5")["text"])
		assert.Equal(t, map[string]any{"width": float64(1024), "height": float64(768), "batch_size": float64(2)}, node("6"))
		sampler := node("7")
		assert.Equal(t, float64(42), sampler["seed"])
		assert.Equal(t, float64(20), sampler["steps"])
		assert.Equal(t, 5.5, sampler["cfg"])
		assert.Equal(t, "euler", sampler["sampler_name"])
		assert.Equal(t, "simple", sampler["scheduler"])
		assert.Equal(t, "new-api/task_abc", node("9")["filename_prefix"])

		turbo := comfyuiCall(t, plugin, []string{"buildSubmitRequest"}, map[string]any{
			"requestBody": map[string]any{"model": "alias", "prompt": "a cat", "n": 1, "width": 512, "height": 512},
			"model":       "alias", "upstreamModel": "comfyui-anima-turbo", "baseUrl": "http://comfy.local:8188", "publicTaskId": "task_t",
		})
		turboWorkflow := turbo["body"].(map[string]any)["prompt"].(map[string]any)
		turboSampler := turboWorkflow["7"].(map[string]any)["inputs"].(map[string]any)
		assert.Equal(t, float64(8), turboSampler["steps"])
		assert.Equal(t, float64(1), turboSampler["cfg"])
		assert.NotNil(t, turboSampler["seed"])
		assert.Equal(t, map[string]any{"Content-Type": "application/json"}, turbo["headers"], "no Authorization header without a channel key")

		_, callErr := plugin.Engine.Call(context.Background(), "buildSubmitRequest", map[string]any{
			"requestBody": map[string]any{"prompt": "a cat"}, "model": "unknown-model", "baseUrl": "http://comfy.local:8188",
		})
		require.ErrorContains(t, callErr, "unsupported ComfyUI workflow model")
	})

	t.Run("parses prompt submission and surfaces validation errors", func(t *testing.T) {
		submitted := comfyuiCall(t, plugin, []string{"parseSubmitResponse"}, driverContext, map[string]any{
			"statusCode": 200, "headers": map[string]any{}, "body": map[string]any{"prompt_id": "7f1c", "number": 3, "node_errors": map[string]any{}},
		})
		assert.Equal(t, "7f1c", submitted["taskId"])
		assert.Equal(t, map[string]any{"pendingPolls": float64(0)}, submitted["state"])

		_, callErr := plugin.Engine.Call(context.Background(), "parseSubmitResponse", driverContext, map[string]any{
			"statusCode": 400, "headers": map[string]any{}, "body": map[string]any{
				"error":       map[string]any{"type": "prompt_outputs_failed_validation", "message": "Prompt outputs failed validation", "details": ""},
				"node_errors": map[string]any{"1": map[string]any{"class_type": "UNETLoader", "errors": []any{map[string]any{"message": "Value not in list", "details": "unet_name: 'missing.safetensors'"}}}},
			},
		})
		require.ErrorContains(t, callErr, "Prompt outputs failed validation")
		require.ErrorContains(t, callErr, "node 1 (UNETLoader): Value not in list: unet_name: 'missing.safetensors'")

		_, callErr = plugin.Engine.Call(context.Background(), "parseSubmitResponse", driverContext, map[string]any{
			"statusCode": 400, "headers": map[string]any{}, "body": map[string]any{
				"error": map[string]any{"type": "invalid_prompt", "message": "Cannot execute because node ETN_LoadImageBase64 does not exist.", "details": "Node ID '#10'"},
			},
		})
		require.ErrorContains(t, callErr, "comfyui-tooling-nodes")
	})

	queryContext := map[string]any{"taskId": "7f1c", "publicTaskId": "task_abc", "baseUrl": "http://comfy.local:8188", "apiKey": "placeholder", "model": "comfyui-anima-aesthetic", "upstreamModel": "comfyui-anima-aesthetic"}
	successOutputs := map[string]any{
		"9":  map[string]any{"images": []any{map[string]any{"filename": "new-api/task_abc_00001_.png", "subfolder": "new-api", "type": "output"}, map[string]any{"filename": "new-api/task_abc_00002_.png", "subfolder": "new-api", "type": "output"}}},
		"10": map[string]any{"images": []any{map[string]any{"filename": "ComfyUI_temp_00001_.png", "subfolder": "", "type": "temp"}}},
	}
	t.Run("polls history until the prompt is terminal", func(t *testing.T) {
		query := comfyuiCall(t, plugin, []string{"buildQueryRequest"}, queryContext)
		assert.Equal(t, "http://comfy.local:8188/history/7f1c", query["url"])
		assert.Equal(t, "GET", query["method"])

		pending := comfyuiCall(t, plugin, []string{"parseTaskResult"}, queryContext, map[string]any{}, map[string]any{"status": 200})
		assert.Equal(t, "IN_PROGRESS", pending["status"])
		assert.Equal(t, map[string]any{"pendingPolls": float64(1)}, pending["state"])

		exhaustedContext := map[string]any{"taskId": "7f1c", "state": map[string]any{"pendingPolls": 240}}
		exhausted := comfyuiCall(t, plugin, []string{"parseTaskResult"}, exhaustedContext, map[string]any{}, map[string]any{"status": 200})
		assert.Equal(t, "FAILURE", exhausted["status"])
		assert.Contains(t, exhausted["reason"], "never reported the prompt")

		success := comfyuiCall(t, plugin, []string{"parseTaskResult"}, queryContext, comfyuiHistory("success", true, successOutputs, nil), map[string]any{"status": 200})
		assert.Equal(t, "SUCCESS", success["status"])

		failed := comfyuiCall(t, plugin, []string{"parseTaskResult"}, queryContext, comfyuiHistory("error", false, map[string]any{}, []any{
			[]any{"execution_start", map[string]any{"prompt_id": "7f1c"}},
			[]any{"execution_error", map[string]any{"node_type": "KSampler", "exception_type": "OutOfMemoryError", "exception_message": "CUDA out of memory"}},
		}), map[string]any{"status": 200})
		assert.Equal(t, "FAILURE", failed["status"])
		assert.Equal(t, "KSampler: CUDA out of memory", failed["reason"])

		empty := comfyuiCall(t, plugin, []string{"parseTaskResult"}, queryContext, comfyuiHistory("success", true, map[string]any{}, nil), map[string]any{"status": 200})
		assert.Equal(t, "FAILURE", empty["status"])
	})

	t.Run("exposes saved images as artifacts served through /view", func(t *testing.T) {
		history := comfyuiHistory("success", true, successOutputs, nil)
		artifactsValue, callErr := plugin.Engine.Call(context.Background(), "listArtifacts", map[string]any{"taskId": "task_abc", "status": "SUCCESS", "data": history})
		require.NoError(t, callErr)
		encoded, marshalErr := common.Marshal(artifactsValue)
		require.NoError(t, marshalErr)
		assert.JSONEq(t, `[{"key":"image-1","type":"image","mimeType":"image/png"},{"key":"image-2","type":"image","mimeType":"image/png"}]`, string(encoded))

		content := comfyuiCall(t, plugin, []string{"buildContentRequest"}, map[string]any{
			"artifactKey": "image-2", "data": history, "upstreamTaskId": "7f1c", "baseUrl": "http://comfy.local:8188", "apiKey": "",
			"clientRequest": map[string]any{"method": "HEAD", "headers": map[string]any{}},
		})
		assert.Equal(t, "http://comfy.local:8188/view?filename=new-api%2Ftask_abc_00002_.png&subfolder=new-api&type=output", content["url"])
		assert.Equal(t, "HEAD", content["method"])

		_, callErr = plugin.Engine.Call(context.Background(), "buildContentRequest", map[string]any{
			"artifactKey": "image-3", "data": history, "upstreamTaskId": "7f1c", "baseUrl": "http://comfy.local:8188",
			"clientRequest": map[string]any{"method": "GET", "headers": map[string]any{}},
		})
		require.ErrorContains(t, callErr, "artifact_not_found")
	})

	t.Run("reports the requested and produced image counts", func(t *testing.T) {
		assert.Equal(t, map[string]any{"image_count": float64(2)}, comfyuiCall(t, plugin, []string{"extractUsage"}, driverContext))
		completed := comfyuiCall(t, plugin, []string{"extractUsageOnComplete"}, queryContext, map[string]any{"status": "SUCCESS"}, comfyuiHistory("success", true, successOutputs, nil))
		assert.Equal(t, map[string]any{"image_count": float64(2)}, completed)
		assert.Empty(t, comfyuiCall(t, plugin, []string{"extractUsageOnComplete"}, queryContext, map[string]any{"status": "FAILURE"}, map[string]any{}))
	})

	t.Run("renders host artifact URLs in generation order", func(t *testing.T) {
		rendererContext := map[string]any{
			"protocol": "openai_images", "stream": false,
			"artifacts": map[string]any{
				"image-2": map[string]any{"key": "image-2", "type": "image", "url": "https://gateway.example/two"},
				"image-1": map[string]any{"key": "image-1", "type": "image", "url": "https://gateway.example/one"},
			},
		}
		task := map[string]any{"task_id": "task_abc", "status": "SUCCESS"}
		images := comfyuiCall(t, plugin, []string{"protocols", "openai_images", "renderFinal"}, rendererContext, task)
		assert.Equal(t, []any{map[string]any{"artifact": "image-1"}, map[string]any{"artifact": "image-2"}}, images["data"], "Images default to host-inlined b64_json")

		urlContext := map[string]any{"protocol": "openai_images", "stream": false, "artifacts": rendererContext["artifacts"],
			"body": map[string]any{"kind": "json", "value": map[string]any{"model": "comfyui-anima-aesthetic", "response_format": "url"}}}
		images = comfyuiCall(t, plugin, []string{"protocols", "openai_images", "renderFinal"}, urlContext, task)
		assert.Equal(t, []any{map[string]any{"url": "https://gateway.example/one"}, map[string]any{"url": "https://gateway.example/two"}}, images["data"])

		multipartContext := map[string]any{"protocol": "openai_images", "stream": false, "artifacts": rendererContext["artifacts"],
			"body": map[string]any{"kind": "multipart", "fields": map[string]any{"response_format": []string{"url"}}, "files": []map[string]any{}}}
		images = comfyuiCall(t, plugin, []string{"protocols", "openai_images", "renderFinal"}, multipartContext, task)
		assert.Equal(t, []any{map[string]any{"url": "https://gateway.example/one"}, map[string]any{"url": "https://gateway.example/two"}}, images["data"])

		final := comfyuiCall(t, plugin, []string{"protocols", "openai_responses", "renderFinal"}, rendererContext, task)
		output := final["output"].([]any)[0].(map[string]any)
		text := output["content"].([]any)[0].(map[string]any)["text"]
		assert.Equal(t, "![Image 1](<https://gateway.example/one>)\n![Image 2](<https://gateway.example/two>)", text)

		events := comfyuiCall(t, plugin, []string{"protocols", "openai_responses", "renderEvents"}, rendererContext, task, nil)
		assert.Equal(t, true, events["done"])
		assert.Len(t, events["events"], 1)

		_, callErr := plugin.Engine.CallPath(context.Background(), "protocols", []string{"openai_images", "renderFinal"}, map[string]any{"artifacts": map[string]any{}}, task)
		require.ErrorContains(t, callErr, "image artifacts are unavailable")
	})
}

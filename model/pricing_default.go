package model

import (
	"regexp"
	"strings"
)

type defaultVendorPreset struct {
	id      int
	name    string
	icon    string
	pattern *regexp.Regexp
}

// These presets are presentation data, never database seed records. Keep
// existing names and negative IDs stable so saved metadata remains authoritative.
// Match platform namespaces and derived model families before their base models:
// @cf/, Nemotron, Sonar, BGE, Hermes, DeepSeek distillations and 360gpt.
// OpenAI comes last: embedding/TTS aliases and an openai/ compatibility prefix
// must not override another model family or publisher.
var defaultVendorPresets = []defaultVendorPreset{
	{
		id: -1003, name: "Cloudflare", icon: "Cloudflare.Color",
		pattern: regexp.MustCompile(`^@cf/`),
	},
	{
		id: -1025, name: "NVIDIA", icon: "Nvidia.Color",
		pattern: regexp.MustCompile(`(?:^|[/.:])nvidia[/.]|\bnemotron(?:[-._]|$)`),
	},
	{
		id: -1024, name: "Perplexity", icon: "Perplexity.Color",
		pattern: regexp.MustCompile(`(?:^|[/.:])perplexity(?:[/.:]|$)|\bsonar(?:[-._]|$)`),
	},
	{
		id: -1031, name: "BAAI", icon: "BAAI",
		pattern: regexp.MustCompile(`(?:^|[/.:])baai/|\bbge-`),
	},
	{
		id: -1037, name: "Nous Research", icon: "NousResearch",
		pattern: regexp.MustCompile(`(?:^|[/.:])nousresearch/|\bhermes-`),
	},
	{
		id: -1001, name: "360", icon: "Ai360.Color",
		pattern: regexp.MustCompile(`(?:^|[/.:])360(?:gpt|zhinao)(?:[0-9]|[-._]|$)`),
	},
	{
		id: -1005, name: "DeepSeek", icon: "DeepSeek.Color",
		pattern: regexp.MustCompile(`(?:^|[/.:])deepseek(?:-ai)?/|\bdeepseek(?:[-._]|$)`),
	},
	{
		id: -1022, name: "阿里巴巴", icon: "Qwen.Color",
		pattern: regexp.MustCompile(`\b(?:qwen(?:[0-9]|[-._/:]|$)|qwq-|qvq-|gte-|tongyi(?:[-._/]|$))|(?:^|[/.:])(?:text-embedding-v[0-9]+|gui-plus|z-image)(?:[-._:]|$)`),
	},
	{
		id: -1029, name: "StepFun", icon: "Stepfun",
		pattern: regexp.MustCompile(`(?:^|[/.:])stepfun(?:-ai)?/|\bstep-`),
	},
	{
		id: -1002, name: "Anthropic", icon: "Claude.Color",
		pattern: regexp.MustCompile(`(?:^|[/.:])anthropic[/.:]|\bclaude(?:[-._]|$)`),
	},
	{
		id: -1006, name: "Google", icon: "Gemini.Color",
		pattern: regexp.MustCompile(`(?:^|[/.:])google/|\b(?:(?:gemini|gemma|learnlm|imagen|veo)(?:[0-9]|[-._]|$)|nano-banana(?:[-._]|$)|palm-)|(?:^|[/.:])(?:aqa|text-embedding-005|text-multilingual-embedding-002)$`),
	},
	{
		id: -1014, name: "xAI", icon: "Grok",
		pattern: regexp.MustCompile(`(?:^|[/.:])x-?ai[/.:]|\b(?:grok(?:[0-9]|[-._]|$)|xai-)`),
	},
	{
		id: -1026, name: "Wan", icon: "Wan",
		pattern: regexp.MustCompile(`(?:^|[/.:])wan-ai/|\bwanx?(?:[0-9]|[-_])`),
	},
	{
		id: -1011, name: "Moonshot", icon: "Moonshot",
		pattern: regexp.MustCompile(`(?:^|[/.:])moonshotai/|\b(?:moonshot|kimi)(?:[-._]|$)`),
	},
	{
		id: -1009, name: "MiniMax", icon: "Minimax.Color",
		pattern: regexp.MustCompile(`(?:^|[/.:])minimaxai/|\b(?:minimax(?:[-._/]|$)|abab[0-9]|hailuo(?:[-._]|$))|^(?:t2v|i2v|s2v)-01(?:-|$)`),
	},
	{
		id: -1016, name: "字节跳动", icon: "Doubao.Color",
		pattern: regexp.MustCompile(`(?:^|[/.:])(?:bytedance|bytedance-seed)/|\b(?:doubao(?:[-._]|$)|seedance(?:[-._]|$)|seedream(?:[-._]|$)|seed-(?:1-|oss-))`),
	},
	{
		id: -1018, name: "智谱", icon: "Zhipu.Color",
		pattern: regexp.MustCompile(`(?:^|[/.:])(?:zhipu|zai-org|thudm)/|\b(?:chatglm(?:[0-9]|[-._]|$)|glm(?:[0-9]|[-._]|$)|cogview(?:[0-9]|[-._]|$)|cogvideo(?:[0-9x]|[-._]|$))`),
	},
	{
		id: -1019, name: "百度", icon: "Wenxin.Color",
		pattern: regexp.MustCompile(`(?:^|[/.:])(?:baidu|paddlepaddle)/|\b(?:ernie(?:[0-9]|[-._]|$)|wenxin(?:[-._]|$))`),
	},
	{
		id: -1023, name: "零一万物", icon: "Yi.Color",
		pattern: regexp.MustCompile(`(?:^|[/.:])01-ai/|\byi(?:[0-9]|[-._]|$)`),
	},
	{
		id: -1021, name: "讯飞", icon: "Spark.Color",
		pattern: regexp.MustCompile(`(?:^|[/.:])iflytek/|\bspark(?:desk|[0-9]|[-._]|$)`),
	},
	{
		id: -1020, name: "腾讯", icon: "Hunyuan.Color",
		pattern: regexp.MustCompile(`(?:^|[/.:])tencent/|\bhunyuan(?:[-._/]|$)`),
	},
	{
		id: -1027, name: "Baichuan", icon: "Baichuan.Color",
		pattern: regexp.MustCompile(`(?:^|[/.:])baichuan-inc/|\bbaichuan(?:[0-9]|[-._]|$)`),
	},
	{
		id: -1028, name: "InternLM", icon: "InternLM.Color",
		pattern: regexp.MustCompile(`\binternlm(?:[0-9]|[-._/]|$)`),
	},
	{
		id: -1030, name: "MiMo", icon: "XiaomiMiMo",
		pattern: regexp.MustCompile(`(?:^|[/.:])xiaomi(?:mimo)?/|\bmimo(?:[-._]|$)`),
	},
	{
		id: -1010, name: "Mistral", icon: "Mistral.Color",
		pattern: regexp.MustCompile(`(?:^|[/.:])mistralai/|\b(?:mistral|mixtral|codestral|ministral|pixtral|magistral|devstral|voxtral)(?:[-._]|$)`),
	},
	{
		id: -1008, name: "Meta", icon: "Meta.Color",
		pattern: regexp.MustCompile(`(?:^|[/.:])(?:meta-llama|meta)/|\bllama(?:[234]|[-._]|$)`),
	},
	{
		id: -1004, name: "Cohere", icon: "Cohere.Color",
		pattern: regexp.MustCompile(`(?:^|[/.:])(?:cohere|cohereforai)/|\b(?:command(?:[-._]|$)|c4ai-aya-|aya-)|\b(?:embed|rerank)-(?:(?:english|multilingual)(?:-light)?-)?v[0-9]+(?:\.[0-9]+)*(?:[-._:]|$)`),
	},
	{
		id: -1007, name: "Jina", icon: "Jina",
		pattern: regexp.MustCompile(`(?:^|[/.:])jinaai/|\bjina-`),
	},
	{
		id: -1032, name: "Black Forest Labs", icon: "Bfl",
		pattern: regexp.MustCompile(`(?:^|[/.:])black-forest-labs/|\bflux(?:[.-]|$)`),
	},
	{
		id: -1033, name: "Microsoft", icon: "Microsoft.Color",
		pattern: regexp.MustCompile(`(?:^|[/.:])microsoft/|\bphi(?:[234]|[-._]|$)`),
	},
	{
		id: -1034, name: "Amazon", icon: "Aws.Color",
		pattern: regexp.MustCompile(`(?:^|[/.:])amazon[/.:]|\b(?:nova-|titan-)`),
	},
	{
		id: -1035, name: "AI21 Labs", icon: "Ai21",
		pattern: regexp.MustCompile(`(?:^|[/.:])ai21(?:labs)?[/.:]|\bjamba(?:[-._]|$)`),
	},
	{
		id: -1036, name: "Stability AI", icon: "Stability.Color",
		pattern: regexp.MustCompile(`(?:^|[/.:])(?:stabilityai|stability)/|\b(?:stable-diffusion|stable-image|sdxl)(?:[-._/]|$)`),
	},
	{
		id: -1038, name: "Midjourney", icon: "Midjourney",
		pattern: regexp.MustCompile(`\bmidjourney(?:[-._/]|$)|(?:^|[/.:])(?:mj[_-]|swap_face(?:[-._]|$))`),
	},
	{
		id: -1017, name: "快手", icon: "Kling.Color",
		pattern: regexp.MustCompile(`\bkling(?:[0-9]|[-._/]|$)`),
	},
	{
		id: -1013, name: "Vidu", icon: "Vidu.Color",
		pattern: regexp.MustCompile(`\bvidu(?:q[0-9]|[0-9]|[-._/]|$)`),
	},
	{
		id: -1039, name: "Suno", icon: "Suno",
		pattern: regexp.MustCompile(`\bsuno(?:[-._/]|$)`),
	},
	{
		id: -1015, name: "即梦", icon: "Jimeng.Color",
		pattern: regexp.MustCompile(`\bjimeng(?:[-._/]|$)`),
	},
	{
		id: -1012, name: "OpenAI", icon: "OpenAI",
		pattern: regexp.MustCompile(`(?:^|[/.:])openai[/.:]|\b(?:gpt-|chatgpt-|codex-|o[134](?:[-.:]|$)|dall-e(?:-|$)|whisper(?:-|$)|tts-1(?:[-.:]|$)|omni-moderation(?:-|$)|text-(?:embedding-(?:ada-|3-)|moderation-|ada-|babbage-|curie-|davinci-)|davinci-|babbage-|computer-use-preview|sora(?:[-._]|$))`),
	},
}

func initDefaultVendorMapping(metaMap map[string]*Model, vendorMap map[int]*Vendor, enableAbilities []AbilityWithChannel) {
	for _, ability := range enableAbilities {
		modelName := ability.Model
		if _, exists := metaMap[modelName]; exists {
			continue
		}

		vendorID := 0
		normalizedName := strings.ToLower(strings.TrimSpace(modelName))
		for _, preset := range defaultVendorPresets {
			if preset.pattern.MatchString(normalizedName) {
				vendorID = getDisplayVendor(preset, vendorMap)
				break
			}
		}
		metaMap[modelName] = &Model{
			ModelName: modelName,
			VendorID:  vendorID,
			Status:    1,
			NameRule:  NameRuleExact,
		}
	}
}

func getDisplayVendor(preset defaultVendorPreset, vendorMap map[int]*Vendor) int {
	for id, vendor := range vendorMap {
		if strings.EqualFold(vendor.Name, preset.name) {
			return id
		}
	}
	vendorMap[preset.id] = &Vendor{Id: preset.id, Name: preset.name, Status: 1, Icon: preset.icon}
	return preset.id
}

package model

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultVendorPresets(t *testing.T) {
	cases := []struct {
		name   string
		icon   string
		id     int // Existing IDs are part of the pricing response contract.
		models []string
	}{
		{"Cloudflare", "Cloudflare.Color", -1003, []string{"@cf/meta/llama-3.1-8b-instruct", "@cf/google/gemma-3-12b-it"}},
		{"NVIDIA", "Nvidia.Color", 0, []string{"nvidia/llama-3.1-nemotron-70b-instruct", "llama-3.1-nemotron-70b-instruct"}},
		{"Perplexity", "Perplexity.Color", 0, []string{"sonar", "perplexity/sonar-pro", "llama-3.1-sonar-small-128k-online"}},
		{"BAAI", "BAAI", 0, []string{"BAAI/bge-m3", "bge-reranker-v2-m3", "BAAI/bge-multilingual-gemma2"}},
		{"Nous Research", "NousResearch", 0, []string{"NousResearch/Hermes-3-Llama-3.1-8B", "nous-hermes-2-mixtral-8x7b-dpo"}},
		{"360", "Ai360.Color", -1001, []string{"360gpt-pro", "360zhinao2-7b-chat"}},
		{"阿里巴巴", "Qwen.Color", -1022, []string{"qwen3-max", "Qwen/Qwen3-32B", "qwq-32b", "qvq-max", "tongyi-intent-detect-v3", "gte-Qwen2-7B-instruct", "text-embedding-v3", "gui-plus", "z-image-turbo"}},
		{"StepFun", "Stepfun", 0, []string{"step-3", "step-tts-mini", "stepfun-ai/step3"}},
		{"OpenAI", "OpenAI", -1012, []string{"codex-auto-review", " CODEX-MINI-LATEST ", "gpt-5-codex", "chatgpt-4o-latest", "openai/gpt-4.1", "o1", "o3-mini", "o4-mini", "o3:latest", "dall-e-3", "whisper-1", "tts-1-hd", "omni-moderation-latest", "text-embedding-3-large", "text-embedding-ada-002", "text-davinci-003", "babbage-002", "computer-use-preview", "sora-2"}},
		{"Anthropic", "Claude.Color", -1002, []string{"claude-sonnet-4-5", "anthropic.claude-3-haiku-20240307-v1:0"}},
		{"Google", "Gemini.Color", -1006, []string{"gemini-2.5-pro", "google/gemma-3-27b-it", "google/text-embedding-005", "text-embedding-005", "text-multilingual-embedding-002", "gemma3:12b", "learnlm-2.0-flash-experimental", "imagen-4.0-generate-001", "veo-3.1-generate-preview", "nano-banana-pro", "palm-2", "aqa"}},
		{"xAI", "Grok", -1014, []string{"grok-4", "x-ai/grok-3", "xai/grok-3"}},
		{"DeepSeek", "DeepSeek.Color", -1005, []string{"deepseek-chat", "deepseek-reasoner", "deepseek-ai/DeepSeek-R1", "DeepSeek-R1-Distill-Qwen-32B", "deepseek-ai/DeepSeek-R1-Distill-Qwen-32B"}},
		{"Wan", "Wan", 0, []string{"wan2.2-t2v", "wanx2.1-t2v", "wanx-v1", "Wan-AI/Wan2.2-T2V-A14B", "alibaba/wan-2.6"}},
		{"Moonshot", "Moonshot", -1011, []string{"moonshot-v1-8k", "kimi-k2", "moonshotai/Kimi-K2-Instruct"}},
		{"MiniMax", "Minimax.Color", -1009, []string{"MiniMax-M2", "MiniMaxAI/MiniMax-M1-80k", "abab6.5s-chat", "hailuo-02", "t2v-01", "i2v-01-live", "s2v-01"}},
		{"字节跳动", "Doubao.Color", -1016, []string{"volcengine/doubao-1-5-pro", "doubao-1-5-pro", "seedance-1-0-pro", "seedream-4-0", "seed-1-6", "ByteDance-Seed/Seed-OSS-36B-Instruct"}},
		{"智谱", "Zhipu.Color", -1018, []string{"glm-4.5", "zai-org/GLM-4.5", "THUDM/chatglm3-6b", "cogview-4", "cogvideox-5b"}},
		{"百度", "Wenxin.Color", -1019, []string{"ernie-4.0", "PaddlePaddle/ERNIE-4.5-21B-A3B-PT", "ERNIE4.5-300B-A47B"}},
		{"零一万物", "Yi.Color", -1023, []string{"yi-large", "01-ai/Yi-1.5-34B-Chat", "yi_34b"}},
		{"讯飞", "Spark.Color", -1021, []string{"spark", "spark-max", "sparkdesk-v3", "Spark4.0-Ultra"}},
		{"腾讯", "Hunyuan.Color", -1020, []string{"hunyuan-pro", "tencent/Hunyuan-A13B-Instruct", "hunyuan-turbos-latest"}},
		{"Baichuan", "Baichuan.Color", 0, []string{"Baichuan2-13B-Chat", "baichuan-inc/Baichuan-M2-32B"}},
		{"InternLM", "InternLM.Color", 0, []string{"internlm3-8b-instruct", "internlm/internlm2_5-7b-chat"}},
		{"MiMo", "XiaomiMiMo", 0, []string{"mimo-v2-flash", "XiaomiMiMo/MiMo-7B-RL"}},
		{"Mistral", "Mistral.Color", -1010, []string{"mistral-large-latest", "mixtral-8x7b", "codestral-latest", "ministral-8b-latest", "pixtral-12b", "magistral-medium", "devstral-small-latest", "voxtral-mini-latest", "openai/mistralai/Devstral-Small-2505"}},
		{"Meta", "Meta.Color", -1008, []string{"llama-3.1-8b-instruct", "meta-llama/Llama-4-Scout-17B-16E-Instruct", "llama3.2"}},
		{"Cohere", "Cohere.Color", -1004, []string{"command-r-plus", "cohere/command-a", "CohereForAI/aya-23-8B", "c4ai-aya-expanse-32b", "embed-v4.0", "embed-english-v3.0", "embed-multilingual-v3.0", "rerank-v3.5", "rerank-english-v3.0", "rerank-multilingual-v3.0"}},
		{"Jina", "Jina", -1007, []string{"jina-embeddings-v3", "jinaai/jina-reranker-v2-base-multilingual"}},
		{"Black Forest Labs", "Bfl", 0, []string{"black-forest-labs/FLUX.1-dev", "flux.1-pro", "flux-pro-1.1"}},
		{"Microsoft", "Microsoft.Color", 0, []string{"microsoft/Phi-4-mini-instruct", "phi-4", "phi3:mini"}},
		{"Amazon", "Aws.Color", 0, []string{"amazon.nova-pro-v1:0", "us.amazon.nova-micro-v1:0", "amazon.titan-embed-text-v2:0", "nova-pro", "titan-embed-text-v2"}},
		{"AI21 Labs", "Ai21", 0, []string{"ai21.jamba-1-5-large-v1:0", "jamba-large-1.7"}},
		{"Stability AI", "Stability.Color", 0, []string{"stabilityai/stable-diffusion-xl-base-1.0", "stable-image-ultra", "sdxl-turbo"}},
		{"Midjourney", "Midjourney", 0, []string{"midjourney", "mj_imagine", "mj-blend", "swap_face"}},
		{"快手", "Kling.Color", -1017, []string{"kling-v2-1", "kling-v2-5-turbo"}},
		{"Vidu", "Vidu.Color", -1013, []string{"viduq2", "vidu-q2", "viduq2-pro", "vidu2.0"}},
		{"Suno", "Suno", 0, []string{"suno-v4", "suno-v5"}},
		{"即梦", "Jimeng.Color", -1015, []string{"jimeng-v3", "jimeng-image-3.0", "volcengine/jimeng_t2i_v40"}},
	}
	meta := make(map[string]*Model)
	vendors := make(map[int]*Vendor)
	var abilities []AbilityWithChannel
	for _, tc := range cases {
		for _, name := range tc.models {
			abilities = append(abilities, AbilityWithChannel{Ability: Ability{Model: name}})
		}
	}
	initDefaultVendorMapping(meta, vendors, abilities)
	for _, tc := range cases {
		for _, name := range tc.models {
			t.Run(name, func(t *testing.T) {
				require.Contains(t, meta, name)
				item := meta[name]
				require.Contains(t, vendors, item.VendorID)
				vendor := vendors[item.VendorID]
				assert.Equal(t, name, item.ModelName)
				assert.Equal(t, tc.name, vendor.Name)
				assert.Equal(t, tc.icon, vendor.Icon)
				assert.Negative(t, item.VendorID)
				if tc.id != 0 {
					assert.Equal(t, tc.id, item.VendorID)
				}
			})
		}
	}
}

func TestDefaultVendorPresetsLeaveUnknownModelsUnassigned(t *testing.T) {
	for _, name := range []string{"", "custom-model", "yiqi-video", "360p-video", "studio123", "foo-o30", "commandment", "wan", "swanky-video", "phiology", "soraish", "t2v-02", "embed", "embedding", "rerank", "text-embedding-custom", "tts-custom", "viduqwerty", "volcengine/custom-model"} {
		t.Run(name, func(t *testing.T) {
			meta := make(map[string]*Model)
			vendors := make(map[int]*Vendor)
			initDefaultVendorMapping(meta, vendors, []AbilityWithChannel{{Ability: Ability{Model: name}}})
			require.Contains(t, meta, name)
			assert.Zero(t, meta[name].VendorID)
			assert.Empty(t, vendors)
		})
	}
}

func TestDefaultVendorPresetsPreserveSavedMetadata(t *testing.T) {
	custom := &Vendor{Id: 7, Name: "oPeNaI", Icon: "Custom.Icon"}
	blank := &Vendor{Id: 8, Name: "Meta", Icon: ""}
	saved := &Model{Id: 9, ModelName: "codex-auto-review", VendorID: 42, Icon: "Saved.Model", Status: 2}
	unassigned := &Model{Id: 10, ModelName: "wan2.2-t2v", VendorID: 0, Status: 1}
	meta := map[string]*Model{saved.ModelName: saved, unassigned.ModelName: unassigned}
	vendors := map[int]*Vendor{7: custom, 8: blank}
	abilities := []AbilityWithChannel{
		{Ability: Ability{Model: saved.ModelName}},
		{Ability: Ability{Model: unassigned.ModelName}},
		{Ability: Ability{Model: "gpt-5"}},
		{Ability: Ability{Model: "llama-4"}},
	}
	initDefaultVendorMapping(meta, vendors, abilities)
	assert.Same(t, saved, meta[saved.ModelName])
	assert.Equal(t, 42, saved.VendorID)
	assert.Equal(t, "Saved.Model", saved.Icon)
	assert.Equal(t, 2, saved.Status)
	assert.Same(t, unassigned, meta[unassigned.ModelName])
	assert.Zero(t, unassigned.VendorID)
	assert.Equal(t, custom.Id, meta["gpt-5"].VendorID)
	assert.Equal(t, blank.Id, meta["llama-4"].VendorID)
	assert.Same(t, custom, vendors[7])
	assert.Equal(t, "Custom.Icon", custom.Icon)
	assert.Same(t, blank, vendors[8])
	assert.Empty(t, blank.Icon)
	assert.Len(t, vendors, 2)
}

func TestPricingDefaultVendorsRemainPresentationData(t *testing.T) {
	resetPricingEndpointTestTables(t)
	saved := &Vendor{Name: "OpenAI", Icon: "Custom.Icon", Status: 1}
	require.NoError(t, DB.Create(saved).Error)
	insertPricingEndpointChannel(t, 701, constant.ChannelTypeOpenAI, dto.ChannelOtherSettings{})
	for _, name := range []string{"codex-auto-review", "wan2.2-t2v", "custom-model"} {
		insertPricingEndpointAbility(t, 701, name)
	}
	InitChannelCache()
	for range 2 {
		InvalidatePricingCache()
		pricing := pricingByModel(GetPricing())
		require.Contains(t, pricing, "codex-auto-review")
		require.Contains(t, pricing, "wan2.2-t2v")
		require.Contains(t, pricing, "custom-model")
		assert.Equal(t, saved.Id, pricing["codex-auto-review"].VendorID)
		assert.Negative(t, pricing["wan2.2-t2v"].VendorID)
		assert.Zero(t, pricing["custom-model"].VendorID)
		byID := make(map[int]PricingVendor)
		for _, vendor := range GetVendors() {
			byID[vendor.ID] = vendor
		}
		assert.Equal(t, "Custom.Icon", byID[saved.Id].Icon)
		assert.Equal(t, "Wan", byID[pricing["wan2.2-t2v"].VendorID].Name)
		assert.Equal(t, "Wan", byID[pricing["wan2.2-t2v"].VendorID].Icon)
		var vendors, models int64
		require.NoError(t, DB.Model(&Vendor{}).Count(&vendors).Error)
		require.NoError(t, DB.Model(&Model{}).Count(&models).Error)
		assert.EqualValues(t, 1, vendors)
		assert.Zero(t, models)
	}
}

// ComfyUI exposes workflows, not models. Each gateway model below selects the
// diffusion weights for a text-to-image, image-to-image or inpainting workflow
// in ComfyUI API ("prompt") format whose prompt, size, batch size and sampler
// settings come from the request. The anima-aesthetic defaults are taken from
// a verified run; the turbo defaults are the usual distilled-model settings
// and may need tuning for a given checkpoint.
//
// Edits need the "comfyui-tooling-nodes" custom node pack
// (https://github.com/Acly/comfyui-tooling-nodes): ETN_LoadImageBase64 lets
// the source image and mask travel inside the workflow JSON, so a single
// /prompt request is enough and no upload step is required.
const WORKFLOWS = {
  "comfyui-anima-aesthetic": { unet: "anima-aesthetic-v1.1.safetensors", steps: 36, cfg: 4.5 },
  "comfyui-anima-base": { unet: "anima-base-v1.0.safetensors", steps: 36, cfg: 4.5 },
  "comfyui-anima-turbo": { unet: "anima-turbo-v1.1.safetensors", steps: 8, cfg: 1.0 },
};
const TEXT_ENCODER = "qwen_3_06b_base.safetensors";
// ComfyUI detects the Qwen3 encoder from the weights; the type value only has
// to be one the CLIPLoader node accepts and matches the verified run.
const TEXT_ENCODER_TYPE = "stable_diffusion";
const VAE = "qwen_image_vae.safetensors";
const DEFAULT_NEGATIVE_PROMPT = "worst quality, low quality, blurry, jpeg artifacts, watermark, signature, text";
const DEFAULT_SAMPLER = "euler_ancestral";
const DEFAULT_SCHEDULER = "simple";
const DEFAULT_SIZE = { width: 1024, height: 1024 };
// Bounds shared by request validation and the billed image_count fact.
const MAX_IMAGES = 4;
const MIN_SIDE = 256;
const MAX_SIDE = 2048;
const SIDE_STEP = 16;
const MAX_STEPS = 150;
const MAX_CFG = 30;
const MAX_PROMPT_CHARS = 4000;
const MAX_IMAGE_BYTES = 25 << 20;
const DEFAULT_IMG2IMG_STRENGTH = 0.75;
const IMAGE_LOADER_NODE = "ETN_LoadImageBase64";
// /history has no entry until the prompt finished, so a prompt that ComfyUI
// dropped (for example after a restart) would poll forever without this cap.
// The host polls every 15 seconds, so 240 empty polls is about one hour.
const MAX_PENDING_POLLS = 240;

export const meta = {
  apiVersion: 1,
  key: "comfyui",
  name: "ComfyUI",
  icon: "ComfyUI.Color",
  description: {
    en: "Image generation and editing through a self-hosted ComfyUI workflow",
    zh: "通过自部署 ComfyUI 工作流生成与编辑图片",
  },
  version: "1.1.0",
  author: { name: "QuantumNous" },
  baseUrl: "http://127.0.0.1:8188",
  auth: "none",
  models: Object.keys(WORKFLOWS),
  fetchMode: "per_task",
  usageSchema: {
    image_count: {
      type: "number",
      unit: "count",
      unitLabel: { en: "image", zh: "张", "zh-TW": "張", fr: "image", ja: "枚", ru: "изображение", vi: "ảnh" },
      description: { en: "Image generation unit price", zh: "图片生成单价" },
    },
  },
  protocols: [
    { name: "openai_images", supports: ["sync"] },
    { name: "openai_responses", supports: ["stream", "sync", "background"] },
  ],
};

function trimmed(value) {
  return String(value || "").trim();
}

function isObject(value) {
  return !!value && typeof value === "object" && !Array.isArray(value);
}

function authHeaders(ctx, json) {
  const headers = {};
  if (json) headers["Content-Type"] = "application/json";
  if (trimmed(ctx.apiKey)) headers.Authorization = "Bearer " + trimmed(ctx.apiKey);
  return headers;
}

function optionalInteger(value, name, min, max) {
  if (value === undefined || value === null) return undefined;
  if (!Number.isInteger(value) || value < min || value > max) throw new Error(name + " must be an integer between " + min + " and " + max);
  return value;
}

function optionalName(value, name) {
  if (value === undefined || value === null) return undefined;
  if (typeof value !== "string" || !/^[A-Za-z0-9_]{1,64}$/.test(value)) throw new Error(name + " must be a ComfyUI option name");
  return value;
}

function decodeSize(value) {
  if (value === undefined || value === null || value === "auto") return DEFAULT_SIZE;
  const match = /^(\d+)x(\d+)$/.exec(trimmed(value));
  if (!match) throw new Error("size must be WIDTHxHEIGHT, for example 1024x1024");
  const width = Number(match[1]);
  const height = Number(match[2]);
  for (const side of [width, height]) {
    if (side < MIN_SIDE || side > MAX_SIDE || side % SIDE_STEP !== 0)
      throw new Error("size sides must be between " + MIN_SIDE + " and " + MAX_SIDE + " and multiples of " + SIDE_STEP);
  }
  return { width: width, height: height };
}

// Shared normalization for the Images and Responses surfaces. OpenAI-only
// fields such as quality or style are accepted and ignored; ComfyUI sampler
// settings are optional top-level extras.
function decodeImageRequest(req, model, prompt, editing) {
  if (!prompt) throw new Error("prompt is required");
  if (prompt.length > MAX_PROMPT_CHARS) throw new Error("prompt must not exceed " + MAX_PROMPT_CHARS + " characters");
  if (req.stream === true) throw new Error("streaming is not supported");
  if (req.response_format !== undefined && req.response_format !== null && req.response_format !== "url" && req.response_format !== "b64_json")
    throw new Error("response_format must be url or b64_json");
  if (req.negative_prompt !== undefined && req.negative_prompt !== null && typeof req.negative_prompt !== "string")
    throw new Error("negative_prompt must be a string");
  if (req.cfg !== undefined && req.cfg !== null && (typeof req.cfg !== "number" || !Number.isFinite(req.cfg) || req.cfg < 0 || req.cfg > MAX_CFG))
    throw new Error("cfg must be a number between 0 and " + MAX_CFG);
  const requestBody = {
    model: model,
    prompt: prompt,
    n: req.n === undefined || req.n === null ? 1 : optionalInteger(req.n, "n", 1, MAX_IMAGES),
  };
  // Edits keep the source dimensions unless a size is requested explicitly.
  if (req.size !== undefined && req.size !== null && req.size !== "auto") {
    const size = decodeSize(req.size);
    requestBody.width = size.width;
    requestBody.height = size.height;
  } else if (!editing) {
    requestBody.width = DEFAULT_SIZE.width;
    requestBody.height = DEFAULT_SIZE.height;
  }
  if (trimmed(req.negative_prompt)) requestBody.negative_prompt = trimmed(req.negative_prompt).slice(0, MAX_PROMPT_CHARS);
  const steps = optionalInteger(req.steps, "steps", 1, MAX_STEPS);
  if (steps !== undefined) requestBody.steps = steps;
  if (typeof req.cfg === "number") requestBody.cfg = req.cfg;
  const seed = optionalInteger(req.seed, "seed", 0, Number.MAX_SAFE_INTEGER);
  if (seed !== undefined) requestBody.seed = seed;
  const sampler = optionalName(req.sampler_name, "sampler_name");
  if (sampler !== undefined) requestBody.sampler_name = sampler;
  const scheduler = optionalName(req.scheduler, "scheduler");
  if (scheduler !== undefined) requestBody.scheduler = scheduler;
  return requestBody;
}

// Multipart form values arrive as string arrays; coerce the numeric fields so
// the shared validator sees the same types as a JSON body.
function multipartRequest(ctx) {
  const fields = isObject(ctx.body.fields) ? ctx.body.fields : {};
  const req = {};
  for (const name of Object.keys(fields)) {
    const values = Array.isArray(fields[name]) ? fields[name] : [];
    if (values.length > 1) throw new Error(name + " must be provided once");
    if (values.length) req[name] = values[0];
  }
  for (const name of ["n", "steps", "seed"]) {
    if (req[name] === undefined) continue;
    if (!/^\d+$/.test(req[name])) throw new Error(name + " must be an integer");
    req[name] = Number(req[name]);
  }
  for (const name of ["cfg", "strength"]) {
    if (req[name] === undefined) continue;
    const value = Number(req[name]);
    if (!Number.isFinite(value)) throw new Error(name + " must be a number");
    req[name] = value;
  }
  if (req.stream !== undefined) req.stream = req.stream === "true";
  return req;
}

// Accepts the first uploaded file for the OpenAI field (also the image[] array
// spelling) or, in JSON bodies, a data URL / raw Base64 string. The returned
// source is either a request-scoped file reference or inline Base64; the
// workflow builder turns both into the loader node's string input.
function imageSource(ctx, req, name, required) {
  if (ctx.body.kind === "multipart") {
    const files = Array.isArray(ctx.body.files) ? ctx.body.files : [];
    const file = files.find(function (item) {
      return isObject(item) && (item.field === name || item.field === name + "[]");
    });
    if (!file) {
      if (required) throw new Error(name + " file is required");
      return undefined;
    }
    if (typeof file.size === "number" && file.size > MAX_IMAGE_BYTES) throw new Error(name + " must not exceed " + MAX_IMAGE_BYTES / (1 << 20) + " MiB");
    const mimeType = trimmed(file.mimeType).toLowerCase();
    if (mimeType && mimeType !== "application/octet-stream" && !/^image\/(png|jpeg|webp)$/.test(mimeType)) throw new Error(name + " must be a PNG, JPEG or WebP image");
    return { ref: file.ref };
  }
  let value = req[name];
  if (Array.isArray(value)) value = value[0];
  if (value === undefined || value === null || value === "") {
    if (required) throw new Error(name + " is required");
    return undefined;
  }
  if (typeof value !== "string") throw new Error(name + " must be a data URL or Base64 string");
  const match = /^data:image\/(?:png|jpeg|webp);base64,(.+)$/i.exec(value.trim());
  const encoded = match ? match[1] : value.trim();
  if (/^https?:\/\//i.test(encoded)) throw new Error(name + " URLs are not supported; send the image as multipart or a data URL");
  if (!/^[A-Za-z0-9+/=\s]+$/.test(encoded)) throw new Error(name + " must be a data URL or Base64 string");
  if (encoded.length > (MAX_IMAGE_BYTES * 4) / 3 + 4) throw new Error(name + " must not exceed " + MAX_IMAGE_BYTES / (1 << 20) + " MiB");
  return { base64: encoded.replace(/\s+/g, "") };
}

function decodeEditRequest(ctx) {
  const req = ctx.body.kind === "multipart" ? multipartRequest(ctx) : jsonRequest(ctx);
  const image = imageSource(ctx, req, "image", true);
  const mask = imageSource(ctx, req, "mask", false);
  if (req.strength !== undefined && req.strength !== null && (typeof req.strength !== "number" || !Number.isFinite(req.strength) || req.strength <= 0 || req.strength > 1))
    throw new Error("strength must be a number greater than 0 and at most 1");
  const requestBody = decodeImageRequest(req, ctx.model, trimmed(req.prompt), true);
  requestBody.image = image;
  if (mask) requestBody.mask = mask;
  requestBody.strength = typeof req.strength === "number" ? req.strength : mask ? 1 : DEFAULT_IMG2IMG_STRENGTH;
  return { kind: "submit", model: ctx.model, action: mask ? "inpaint" : "image_to_image", requestBody: requestBody };
}

function jsonRequest(ctx) {
  if (!ctx.body || ctx.body.kind !== "json") throw new Error("JSON body required");
  if (!isObject(ctx.body.value)) throw new Error("request body must be an object");
  return ctx.body.value;
}

// The loader node's string input: a host file placeholder (raw Base64, the
// node does not strip data URL prefixes) or Base64 that arrived inline.
function loaderInput(source) {
  if (isObject(source) && typeof source.ref === "string") return { __fileRef: source.ref, encoding: "base64", maxBytes: MAX_IMAGE_BYTES };
  if (isObject(source) && typeof source.base64 === "string") return source.base64;
  throw new Error("image source is missing");
}

// Image-to-image and inpainting latent: encode the source, optionally resized
// to the requested size, and confine denoising to the transparent mask area.
// OpenAI masks mark the edit region with transparency while ETN_LoadImageBase64
// reports alpha as-is, so the mask is inverted before it reaches the latent.
function editLatentNodes(workflow, req, n) {
  const scaled = Number.isInteger(req.width) && Number.isInteger(req.height);
  workflow["10"] = { class_type: IMAGE_LOADER_NODE, inputs: { image: loaderInput(req.image) } };
  let pixels = ["10", 0];
  if (scaled) {
    workflow["11"] = { class_type: "ImageScale", inputs: { image: pixels, upscale_method: "lanczos", width: req.width, height: req.height, crop: "center" } };
    pixels = ["11", 0];
  }
  workflow["6"] = { class_type: "VAEEncode", inputs: { pixels: pixels, vae: ["3", 0] } };
  let latent = ["6", 0];
  if (req.mask) {
    workflow["12"] = { class_type: IMAGE_LOADER_NODE, inputs: { image: loaderInput(req.mask) } };
    workflow["13"] = { class_type: "InvertMask", inputs: { mask: ["12", 1] } };
    let mask = ["13", 0];
    if (scaled) {
      workflow["14"] = { class_type: "MaskToImage", inputs: { mask: mask } };
      workflow["15"] = { class_type: "ImageScale", inputs: { image: ["14", 0], upscale_method: "bilinear", width: req.width, height: req.height, crop: "center" } };
      workflow["16"] = { class_type: "ImageToMask", inputs: { image: ["15", 0], channel: "red" } };
      mask = ["16", 0];
    }
    workflow["17"] = { class_type: "SetLatentNoiseMask", inputs: { samples: latent, mask: mask } };
    latent = ["17", 0];
  }
  if (n > 1) {
    workflow["18"] = { class_type: "RepeatLatentBatch", inputs: { samples: latent, amount: n } };
    latent = ["18", 0];
  }
  return latent;
}

export function buildSubmitRequest(ctx) {
  const req = ctx.requestBody || {};
  const workflowModel = ctx.upstreamModel || ctx.model;
  const template = WORKFLOWS[workflowModel];
  if (!template) throw new Error("unsupported ComfyUI workflow model: " + workflowModel);
  if (!trimmed(req.prompt)) throw new Error("field prompt is required");
  const editing = ctx.action === "image_to_image" || ctx.action === "inpaint" || isObject(req.image);
  const n = Number.isInteger(req.n) && req.n >= 1 && req.n <= MAX_IMAGES ? req.n : 1;
  const size = Number.isInteger(req.width) && Number.isInteger(req.height) ? { width: req.width, height: req.height } : DEFAULT_SIZE;
  const seed = Number.isInteger(req.seed) ? req.seed : Math.floor(Math.random() * Number.MAX_SAFE_INTEGER);
  const strength = typeof req.strength === "number" && req.strength > 0 && req.strength <= 1 ? req.strength : req.mask ? 1 : DEFAULT_IMG2IMG_STRENGTH;
  const workflow = {
    "1": { class_type: "UNETLoader", inputs: { unet_name: template.unet, weight_dtype: "default" } },
    "2": { class_type: "CLIPLoader", inputs: { clip_name: TEXT_ENCODER, type: TEXT_ENCODER_TYPE, device: "default" } },
    "3": { class_type: "VAELoader", inputs: { vae_name: VAE } },
    "4": { class_type: "CLIPTextEncode", inputs: { text: trimmed(req.prompt), clip: ["2", 0] } },
    "5": { class_type: "CLIPTextEncode", inputs: { text: trimmed(req.negative_prompt) || DEFAULT_NEGATIVE_PROMPT, clip: ["2", 0] } },
    "6": { class_type: "EmptyLatentImage", inputs: { width: size.width, height: size.height, batch_size: n } },
    "7": {
      class_type: "KSampler",
      inputs: {
        seed: seed,
        steps: Number.isInteger(req.steps) ? req.steps : template.steps,
        cfg: typeof req.cfg === "number" ? req.cfg : template.cfg,
        sampler_name: req.sampler_name || DEFAULT_SAMPLER,
        scheduler: req.scheduler || DEFAULT_SCHEDULER,
        denoise: editing ? strength : 1.0,
        model: ["1", 0],
        positive: ["4", 0],
        negative: ["5", 0],
        latent_image: ["6", 0],
      },
    },
    "8": { class_type: "VAEDecode", inputs: { samples: ["7", 0], vae: ["3", 0] } },
    // The public task id in the file prefix keeps ComfyUI's output folder
    // traceable to gateway tasks.
    "9": { class_type: "SaveImage", inputs: { filename_prefix: "new-api/" + trimmed(ctx.publicTaskId || "image"), images: ["8", 0] } },
  };
  if (editing) workflow["7"].inputs.latent_image = editLatentNodes(workflow, req, n);
  return {
    url: ctx.baseUrl + "/prompt",
    method: "POST",
    headers: authHeaders(ctx, true),
    body: { prompt: workflow, client_id: utils.uuid() },
  };
}

function comfyErrorMessage(body, statusCode) {
  const parts = [];
  if (isObject(body.error) && trimmed(body.error.message)) {
    parts.push(trimmed(body.error.message) + (trimmed(body.error.details) ? ": " + trimmed(body.error.details) : ""));
  }
  const nodeErrors = isObject(body.node_errors) ? body.node_errors : {};
  for (const nodeId of Object.keys(nodeErrors)) {
    const node = nodeErrors[nodeId] || {};
    for (const error of Array.isArray(node.errors) ? node.errors : []) {
      const detail = trimmed(error && error.details);
      parts.push("node " + nodeId + (node.class_type ? " (" + node.class_type + ")" : "") + ": " + trimmed(error && error.message) + (detail ? ": " + detail : ""));
    }
  }
  if (!parts.length) parts.push("ComfyUI returned HTTP " + statusCode);
  const message = "ComfyUI rejected the workflow: " + parts.join("; ");
  if (message.indexOf(IMAGE_LOADER_NODE) >= 0) return message + " (image edits require the comfyui-tooling-nodes custom node pack on the ComfyUI server)";
  return message;
}

export function parseSubmitResponse(ctx, resp) {
  const body = isObject(resp.body) ? resp.body : {};
  const promptId = trimmed(body.prompt_id);
  if (resp.statusCode >= 400 || !promptId) throw new Error(comfyErrorMessage(body, resp.statusCode));
  return { taskId: promptId, taskData: { prompt_id: promptId, number: body.number }, state: { pendingPolls: 0 } };
}

export function buildQueryRequest(ctx) {
  return { url: ctx.baseUrl + "/history/" + encodeURIComponent(ctx.taskId), method: "GET", headers: authHeaders(ctx, false) };
}

// /history/{prompt_id} answers {"<prompt_id>": {prompt, outputs, status}}; the
// gateway task id differs from the prompt id, so fall back to the only entry.
function historyEntry(body, taskId) {
  if (!isObject(body)) return null;
  if (isObject(body[taskId]) && (body[taskId].outputs || body[taskId].status)) return body[taskId];
  for (const key of Object.keys(body)) {
    if (isObject(body[key]) && (body[key].outputs || body[key].status)) return body[key];
  }
  return null;
}

// Saved images in node order; preview nodes write type "temp" and are skipped.
function outputImages(entry) {
  const outputs = entry && isObject(entry.outputs) ? entry.outputs : {};
  const nodeIds = Object.keys(outputs).sort(function (left, right) {
    return Number(left) - Number(right);
  });
  const images = [];
  for (const nodeId of nodeIds) {
    const files = Array.isArray(outputs[nodeId].images) ? outputs[nodeId].images : [];
    for (const file of files) {
      if (isObject(file) && trimmed(file.filename) && (file.type === undefined || file.type === "output")) images.push(file);
    }
  }
  return images;
}

function executionError(status) {
  for (const message of Array.isArray(status.messages) ? status.messages : []) {
    if (!Array.isArray(message) || message.length < 2 || !isObject(message[1])) continue;
    if (message[0] === "execution_error") {
      const detail = message[1];
      const prefix = trimmed(detail.node_type) ? trimmed(detail.node_type) + ": " : "";
      return prefix + (trimmed(detail.exception_message) || trimmed(detail.exception_type) || "execution error");
    }
    if (message[0] === "execution_interrupted") return "execution was interrupted";
  }
  return "ComfyUI reported an execution error";
}

export function parseTaskResult(ctx, body) {
  const entry = historyEntry(body, ctx.taskId);
  if (!entry) {
    const pending = Number((ctx.state || {}).pendingPolls || 0) + 1;
    if (pending > MAX_PENDING_POLLS)
      return { status: "FAILURE", reason: "ComfyUI never reported the prompt; it may have been dropped by a restart", state: { pendingPolls: pending } };
    // Queued and running prompts are indistinguishable through /history.
    return { status: "IN_PROGRESS", state: { pendingPolls: pending } };
  }
  const status = isObject(entry.status) ? entry.status : {};
  if (status.status_str === "error") return { status: "FAILURE", reason: executionError(status) };
  if (status.status_str === "success" || status.completed === true) {
    if (!outputImages(entry).length) return { status: "FAILURE", reason: "the workflow finished without saving an image" };
    return { status: "SUCCESS", progress: "100%" };
  }
  return { status: "IN_PROGRESS" };
}

export function extractUsage(ctx) {
  const n = Number((ctx.requestBody || {}).n);
  return { image_count: Number.isInteger(n) && n >= 1 && n <= MAX_IMAGES ? n : 1 };
}

export function extractUsageOnComplete(task, taskResult, body) {
  if (!taskResult || taskResult.status !== "SUCCESS") return {};
  const count = outputImages(historyEntry(body, task.taskId)).length;
  return count > 0 ? { image_count: Math.min(count, MAX_IMAGES) } : {};
}

function mimeTypeFor(filename) {
  const extension = trimmed(filename).toLowerCase().split(".").pop();
  if (extension === "jpg" || extension === "jpeg") return "image/jpeg";
  if (extension === "webp") return "image/webp";
  return "image/png";
}

export function listArtifacts(task) {
  if (task.status !== "SUCCESS") return [];
  return outputImages(historyEntry(task.data, task.taskId)).map(function (file, index) {
    return { key: "image-" + (index + 1), type: "image", mimeType: mimeTypeFor(file.filename) };
  });
}

export function buildContentRequest(ctx) {
  const match = /^image-(\d+)$/.exec(String(ctx.artifactKey || ""));
  const file = match ? outputImages(historyEntry(ctx.data, ctx.upstreamTaskId))[Number(match[1]) - 1] : undefined;
  if (!file) throw new Error("artifact_not_found");
  return {
    url:
      ctx.baseUrl +
      "/view?filename=" +
      encodeURIComponent(file.filename) +
      "&subfolder=" +
      encodeURIComponent(trimmed(file.subfolder)) +
      "&type=" +
      encodeURIComponent(trimmed(file.type) || "output"),
    method: ctx.clientRequest.method,
    headers: authHeaders(ctx, false),
  };
}

// Host-injected artifacts keyed image-N, returned in generation order.
function artifactImages(ctx) {
  const artifacts = ctx && isObject(ctx.artifacts) ? ctx.artifacts : {};
  const images = [];
  for (const key of Object.keys(artifacts)) {
    const match = /^image-(\d+)$/.exec(key);
    if (match && trimmed(artifacts[key] && artifacts[key].url)) images.push({ index: Number(match[1]), key: key, url: trimmed(artifacts[key].url) });
  }
  images.sort(function (left, right) {
    return left.index - right.index;
  });
  if (!images.length) throw new Error("image artifacts are unavailable");
  return images;
}

// Images responses default to inline b64_json because the gateway artifact
// URL is often not reachable from where the client forwards the result;
// response_format "url" opts into the signed gateway URLs instead.
function imagesResponseFormat(ctx) {
  const body = ctx && isObject(ctx.body) ? ctx.body : {};
  let value;
  if (body.kind === "json" && isObject(body.value)) value = body.value.response_format;
  else if (body.kind === "multipart" && isObject(body.fields)) value = (body.fields.response_format || [])[0];
  return value === "url" ? "url" : "b64_json";
}

function responsesImageText(ctx) {
  return artifactImages(ctx)
    .map(function (image, index) {
      return "![Image " + (index + 1) + "](<" + image.url + ">)";
    })
    .join("\n");
}

function responsesInput(req) {
  const texts = [];
  const input = req.input;
  if (typeof input === "string") texts.push(input);
  else if (Array.isArray(input)) {
    for (const item of input) {
      if (typeof item === "string") {
        texts.push(item);
        continue;
      }
      if (!isObject(item)) continue;
      const content = item.content === undefined ? [item] : Array.isArray(item.content) ? item.content : [item.content];
      for (const part of content) {
        if (typeof part === "string") texts.push(part);
        else if (isObject(part) && ["input_text", "text"].includes(part.type) && typeof part.text === "string") texts.push(part.text);
      }
    }
  }
  return texts
    .filter(function (text) {
      return trimmed(text);
    })
    .join("\n");
}

export const protocols = {
  openai_images: {
    decodeRequest: function (ctx) {
      if (ctx.operation === "edit") return decodeEditRequest(ctx);
      const req = jsonRequest(ctx);
      const requestBody = decodeImageRequest(req, ctx.model, trimmed(req.prompt), false);
      return { kind: "submit", model: ctx.model, action: "text_to_image", requestBody: requestBody };
    },
    renderFinal: function (ctx, _task) {
      const format = imagesResponseFormat(ctx);
      return {
        data: artifactImages(ctx).map(function (image) {
          return format === "url" ? { url: image.url } : { artifact: image.key };
        }),
      };
    },
  },
  openai_responses: {
    decodeRequest: function (ctx) {
      const req = jsonRequest(ctx);
      const model = trimmed(req.model);
      if (!model) throw new Error("model is required");
      if (req.input !== undefined && typeof req.input !== "string" && !Array.isArray(req.input)) throw new Error("input must be a string or array");
      const prompt = responsesInput(req) || trimmed(req.prompt);
      if (!prompt) throw new Error("input is required");
      const requestBody = decodeImageRequest(req, model, prompt, false);
      return { kind: "submit", model: model, action: "text_to_image", requestBody: requestBody };
    },
    renderEvents: function (ctx, task, previousState) {
      const status = String(task.status || "UNKNOWN").toUpperCase();
      const state = { status: status };
      if (status === "SUCCESS") {
        const events = previousState && previousState.status === status ? [] : [{ type: "output", data: responsesImageText(ctx) }];
        return { events: events, state: state, done: true };
      }
      if (status === "FAILURE")
        return { events: [{ type: "error", code: "task_failed", message: task.fail_reason || "task failed" }], state: state, done: true };
      if (previousState && previousState.status === status) return { events: [], state: state, done: false };
      return { events: [{ type: "progress", message: status.toLowerCase() }], state: state, done: false };
    },
    renderFinal: function (ctx, _task) {
      return {
        output: [
          {
            type: "message",
            status: "completed",
            role: "assistant",
            content: [{ type: "output_text", text: responsesImageText(ctx), annotations: [], logprobs: [] }],
          },
        ],
        metadata: { vendor: "comfyui" },
      };
    },
  },
};

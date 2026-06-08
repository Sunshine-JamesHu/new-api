---
name: openai-media-node
description: Generate OpenAI images and multi-provider videos with Node.js scripts. Use when the user asks Codex to create or update scripts/workflows for gpt-image-2 image generation with the official OpenAI Node SDK, Sora/OpenAI video generation, Doubao Seedance video, Kling video, HappyHorse video, text-to-video, image-to-video, reference-to-video, video editing, media job polling/downloading, or separate API keys for image, video, and inference tasks.
---

# OpenAI Media Node

Use this skill when building or running media scripts in Node.js.

## Setup

1. Work from this skill directory:

```bash
cd .agents/skills/openai-media-node
```

2. Install dependencies once:

```bash
npm install
```

3. Copy `.env.example` to `.env` and fill the two keys:

```bash
VIDEO_API_KEY=sk-...
IMAGE_API_KEY=sk-...
NEWAPI_BASE_URL=http://localhost:3000
```

All scripts are intended to go through your NewApi deployment. Video generation and inference use `VIDEO_API_KEY`; image generation uses `IMAGE_API_KEY`.

## Scripts

- `scripts/generate-image.mjs`: image generation with `gpt-image-2`.
- `scripts/generate-video.mjs`: OpenAI Sora video generation, polling, and MP4 download.
- `scripts/newapi-video.mjs`: new-api OpenAI-compatible video create/get workflow using axios.
- `scripts/provider-video.mjs`: optional reference script for provider-native HTTP APIs; prefer `newapi-video.mjs` for this repo.
- `scripts/infer.mjs`: Responses API inference with the inference key.

## Image Workflow

Use `gpt-image-2` by default. Prefer the Image API for one-shot image generation.

```bash
node scripts/generate-image.mjs \
  --prompt "A cinematic product photo of a matte black espresso machine" \
  --out outputs/espresso.png \
  --size 1024x1024 \
  --quality high
```

Supported common flags: `--model`, `--prompt`, `--out`, `--size`, `--quality`, `--format`, `--compression`, `--background`, `--moderation`, `--n`, `--stream`, `--partial-images`.

## OpenAI Video Workflow

Use the official Videos API parameter names. Default model is `sora-2`; use `sora-2-pro` for production quality and 1080p exports.

```bash
node scripts/generate-video.mjs \
  --prompt "Wide tracking shot of a teal coupe driving through a desert highway, hard sun overhead." \
  --out outputs/highway.mp4 \
  --model sora-2-pro \
  --size 1280x720 \
  --seconds 8
```

Supported common flags: `--model`, `--prompt`, `--out`, `--size`, `--seconds`, `--input-reference`, `--input-reference-url`, `--input-reference-file-id`, `--characters-json`, `--poll-interval`, `--timeout`.

For image references, the input image should match the target video `size`. Supported reference formats are JPEG, PNG, and WebP.

## Provider Video Workflow

Prefer `scripts/newapi-video.mjs` when testing this repository or any new-api deployment. It uses the official new-api video endpoints:

- `POST /v1/video/generations`
- `GET /v1/video/generations/{task_id}`

The script keeps `duration` at the top level so per-second billing uses the trusted value implemented in this repo. Provider-specific fields go into `metadata`.

```bash
node scripts/newapi-video.mjs \
  --provider doubao \
  --mode i2v \
  --model doubao-seedance-1-0-lite-i2v \
  --prompt "Camera slowly pushes in." \
  --image-url https://example.com/first-frame.png \
  --duration 5 \
  --base-url http://localhost:3000
```

For Doubao, Kling, and HappyHorse through NewApi, always choose the mode first:

- `t2v`: text-to-video.
- `i2v`: image-to-video / first-frame-to-video.
- `r2v`: reference-image/video-to-video.
- `edit`: video editing.

Examples:

```bash
node scripts/provider-video.mjs \
  --provider doubao \
  --model doubao-seedance-1-0-lite-i2v \
  --mode i2v \
  --prompt "Camera slowly pushes in." \
  --image-url https://example.com/first-frame.png \
  --duration 5 \
  --base-url https://ark.cn-beijing.volces.com
```

```bash
node scripts/provider-video.mjs \
  --provider happyhorse \
  --model happyhorse-1.0-r2v \
  --mode r2v \
  --prompt "Keep the product consistent while rotating the camera." \
  --media-json "[{\"type\":\"reference_image\",\"url\":\"https://example.com/ref-a.png\"}]" \
  --duration 6 \
  --resolution 1080P \
  --base-url https://dashscope.aliyuncs.com
```

```bash
node scripts/provider-video.mjs \
  --provider kling \
  --model kling-v2-master \
  --mode i2v \
  --prompt "The subject turns toward camera." \
  --image-url https://example.com/start.png \
  --duration 5 \
  --base-url https://api.klingai.com
```

The NewApi script accepts `--extra-json`, `--metadata-json`, `--input-json`, `--parameters-json`, and `--content-json` so official fields from the docs can be passed through without changing the script. See `references/newapi-video-api.md` and `references/provider-video-modes.md` before adding or changing provider-specific behavior.

## Inference Workflow

Use the inference key for planning, prompt expansion, or metadata generation:

```bash
node scripts/infer.mjs \
  --input "Write a concise Sora prompt for a coffee cup reveal shot." \
  --model gpt-5.5
```

## Official References

Read `references/openai-media-api.md` for OpenAI Images/Sora. Read `references/newapi-video-api.md` for this repo's OpenAI-compatible video API. Read `references/provider-video-modes.md` for Doubao/Kling/HappyHorse mode mapping and provider payloads. If official docs may have changed, verify against the provider docs before editing scripts.

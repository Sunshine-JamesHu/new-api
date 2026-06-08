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

Doubao examples:

```bash
# Text to video. The script flag writes provider metadata.generate_audio.
node scripts/newapi-video.mjs \
  --provider doubao \
  --mode t2v \
  --model doubao-seedance-2-0-fast-260128 \
  --prompt "FPV drone aerial shot over a misty mountain valley." \
  --duration 4 \
  --resolution 720P \
  --ratio 16:9 \
  --generate-audio false

# Vertical first-frame image to video, 5 seconds, 720p.
node scripts/newapi-video.mjs \
  --provider doubao \
  --mode i2v \
  --model doubao-seedance-2-0-fast-260128 \
  --prompt "Sea breeze across her face, hair and skirt moving gently." \
  --first-frame-file ./inputs/person-by-the-sea.jpg \
  --duration 5 \
  --resolution 720P \
  --ratio 9:16 \
  --generate-audio false

# Single first-frame image to video.
node scripts/newapi-video.mjs \
  --provider doubao \
  --mode i2v \
  --model doubao-seedance-2-0-fast-260128 \
  --prompt "Smooth FPV drone push forward from the first frame." \
  --first-frame-file ./inputs/start.jpg \
  --duration 4 \
  --resolution 720P

# First/last-frame video.
node scripts/newapi-video.mjs \
  --provider doubao \
  --mode i2v \
  --model doubao-seedance-2-0-fast-260128 \
  --prompt "Fly from the snowy mountain opening to the coastal sunset ending." \
  --first-frame-file ./inputs/start.jpg \
  --last-frame-file ./inputs/end.jpg \
  --duration 4 \
  --resolution 720P

# Multi-reference video.
node scripts/newapi-video.mjs \
  --provider doubao \
  --mode r2v \
  --model doubao-seedance-2-0-fast-260128 \
  --prompt "Keep the landscape style consistent while flying forward." \
  --reference-image-files "./inputs/ref-a.jpg;./inputs/ref-b.jpg;./inputs/ref-c.jpg" \
  --duration 4 \
  --resolution 720P

# Seedance 2.0 multimodal reference: images plus optional audio.
node scripts/newapi-video.mjs \
  --provider doubao \
  --mode r2v \
  --model doubao-seedance-2-0-fast-260128 \
  --prompt "Use the references for character, outfit, and seaside mood." \
  --reference-image-files "./inputs/ref-a.jpg;./inputs/ref-b.jpg" \
  --reference-audio-file ./inputs/voice.wav \
  --duration 5 \
  --resolution 720P \
  --ratio 9:16 \
  --generate-audio true
```

Doubao requests use official Ark fields inside `metadata.content`: `text`, `image_url`, `video_url`, `audio_url`, and optional `draft_task`. The script sets image roles for first frame, last frame, and reference images. It also supports official fields such as `--return-last-frame`, `--service-tier`, `--execution-expires-after`, `--frames`, `--seed`, `--camera-fixed`, `--watermark`, `--draft`, `--draft-task-id`, `--web-search`, `--safety-identifier`, and `--priority`.

Keep billable/render duration at top level with `--duration` or `--seconds`. Provider fields go in `metadata`; `--generate-audio` is metadata-only because the gateway does not support top-level `generate_audio`.

Important Doubao limits from Ark docs: Seedance 2.0 fast does not support `1080p`; `duration` is integer seconds and defaults to 5; Seedance 2.0/1.5 support audio generation, defaulting upstream to true when not specified; Seedance 2.0 reference mode can use 1-9 images, up to 3 reference videos, and up to 3 reference audios, but audio cannot be the only reference input. Seedance 2.0 rejects direct upload of many real-person face references; surface that upstream error to the caller instead of retrying blindly.

For Doubao, Kling, and HappyHorse through NewApi, always choose the mode first:

- `t2v`: text-to-video.
- `i2v`: image-to-video / first-frame-to-video.
- `r2v`: reference-image/video-to-video.
- `edit`: video editing.

Image input flags:

- URL flags: `--image-url`, `--first-frame-url`, `--last-frame-url`, `--reference-image-url`.
- Local file flags: `--image-file`, `--first-frame-file`, `--last-frame-file`, `--reference-image-file`.
- Multiple local references: `--reference-image-files "a.jpg;b.jpg;c.jpg"`.
- Video/audio flags: `--video-url`, `--video-file`, `--reference-video-url`, `--reference-video-file`, `--audio-url`, `--audio-file`, `--reference-audio-url`, `--reference-audio-file`.

HappyHorse requests default to `watermark: false`; pass `--watermark true` only when the upstream watermark is desired.

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

---
name: owl-ai
description: Call a customer's deployed new-api/OwlAI site for image generation, video generation, video task polling/downloading, and optional inference helpers. Use when Codex needs to create or run scripts that exercise /v1/images/generations or /v1/video/generations. For image generation, the first local command/tool timeout must be at least 600000 ms; never start with a 120-second timeout.
---

# OwlAI

For a beginner-friendly setup walkthrough, read `CONFIGURATION.md`. It covers Windows installation, `.env` variables, image generation, video generation, and troubleshooting against `https://www.owlai.top`.

Use this skill when writing or running customer-facing media API examples for a deployed new-api/OwlAI site. The scripts call the customer's gateway. They do not call upstream vendors directly.

## Required Tool Timeouts

Image generation is a long-running synchronous customer API call. When running `scripts/generate-image.mjs` from Codex, set the shell/tool `timeout_ms` to at least `600000` on the first attempt, and pass `--request-timeout 600000` unless the user asks for a different value. Do not start with a 120-second local timeout.

If a local tool timeout already happened, treat it as an unknown local execution state, not as image generation failure. Do not immediately submit a second generation request. First check for the expected `--response-out` JSON, output image path, and any still-running `node` process for the original command.

If the response JSON and image file were written well before the local timeout, the generation finished and only the local Node process failed to exit promptly. Use the current `generate-image.mjs`, which exits explicitly after writing outputs, and do not bill the customer with a duplicate request.

All Node CLI scripts in this skill must explicitly terminate after `main()` resolves or rejects. Use `process.exit(0)` on success and `process.exit(1)` after `handleError(error)` on failure, so lingering HTTP client handles cannot cause false local timeouts after outputs have already been written.

## Setup

1. Work from this skill directory:

```bash
cd .agents/skills/owl-ai
```

2. Install dependencies once:

```bash
npm install
```

3. Copy `.env.example` to `.env` and fill the deployed site URL and customer tokens:

```bash
NEWAPI_BASE_URL=https://your-new-api.example.com
IMAGE_API_KEY=sk-customer-image-token
SEEDANCE_API_KEY=sk-customer-seedance-token
HAPPYHORSE_API_KEY=sk-customer-happyhorse-token
```

`NEWAPI_BASE_URL` is the customer's deployed new-api/OwlAI site. `IMAGE_API_KEY`, `SEEDANCE_API_KEY`, and `HAPPYHORSE_API_KEY` are tokens issued by that site. Video requests select their key automatically: any model containing `seedance` uses `SEEDANCE_API_KEY`; HappyHorse requests (`--provider happyhorse`, or models containing `happyhorse`) use `HAPPYHORSE_API_KEY`. Other video providers can continue to use an optional `VIDEO_API_KEY`. When polling an existing Seedance or HappyHorse task, pass the same `--model` or `--provider` so the script can select its key. Optional `IMAGE_API_BASE_URL` and `VIDEO_API_BASE_URL` may override the shared base URL for separate deployments.

## Scripts

- `scripts/generate-image.mjs`: calls `{NEWAPI_BASE_URL}/v1/images/generations`.
- `scripts/newapi-video.mjs`: creates and polls video tasks through `{NEWAPI_BASE_URL}/v1/video/generations`.
- `scripts/infer.mjs`: calls `{NEWAPI_BASE_URL}/v1/responses` for prompt drafting or metadata helpers.

## Image Workflow

Use the deployed site's image generation endpoint. The script sends JSON over HTTP and saves either returned base64 image data or image URLs.

Image generation can legitimately take several minutes. When running this script through Codex tools, set the local command timeout to at least 10 minutes on the first attempt. Do not use a 120-second local timeout and then retry if the gateway may already have completed the image request.

```bash
node scripts/generate-image.mjs \
  --model gpt-image-2 \
  --prompt "A cinematic product photo of a matte black espresso machine" \
  --out outputs/espresso.png \
  --size 1024x1024 \
  --quality high \
  --request-timeout 600000
```

Supported common flags: `--model`, `--prompt`, `--prompt-file`, `--out`, `--size`, `--quality`, `--format`, `--compression`, `--background`, `--moderation`, `--input-fidelity`, `--n`, `--base-url`, `--request-timeout`, `--dry-run`, `--request-out`, `--response-out`.

### Reference-image generation (multi-image)

The script builds a single `images` array and sends it to the gateway's `/v1/images/generations`, mirroring the imagegen skill's multi-image reference workflow. Local files are converted to `data:image/...;base64,...` data URLs; remote URLs pass through unchanged. Use this when the model supports reference images (for example GPT Image edit-style reference workflows).

Text + one local reference:

```bash
node scripts/generate-image.mjs \
  --model gpt-image-2 \
  --prompt "Recreate this product in the same style." \
  --image-file ./inputs/product.png \
  --out outputs/product-variant.png
```

Multiple local references (semicolon or comma separated, or repeat the flag):

```bash
node scripts/generate-image.mjs \
  --model gpt-image-2 \
  --prompt "Combine the character and the scene into one frame." \
  --image-files "./inputs/char.png;./inputs/scene.png" \
  --out outputs/combined.png
```

Mixed local files and remote URLs:

```bash
node scripts/generate-image.mjs \
  --model gpt-image-2 \
  --prompt "Keep the character and background consistent." \
  --image-files "./inputs/char.png" \
  --image-urls "https://example.com/bg.png;https://example.com/lighting.png" \
  --out outputs/hero.png
```

Reference-image flags:

- Local files: `--image-file`, `--image-files`, `--reference-image-file`, `--reference-image-files`, `--images`, `--reference-images`.
- Remote URLs: `--image-url`, `--image-urls`, `--reference-image-url`, `--reference-image-urls`.
- Values may be `;`- or `,`-separated, and flags may be repeated. Local file paths are converted to data URLs; `data:image/...` and `http(s)://` values are passed through as-is.
- Single `--image`/`--reference-image` sets a single reference (a local path becomes a data URL); a bare remote URL is sent as `image` for backward compatibility.
- Reference order is preserved and meaningful: describe each image by index and role in the prompt.
- Passing reference images is only meaningful when the selected model/gateway actually consumes them; plain text-to-image models ignore them.

Preview the exact request body without calling the API:

```bash
node scripts/generate-image.mjs \
  --model gpt-image-2 \
  --prompt "test" \
  --image-files "./inputs/a.png;./inputs/b.png" \
  --dry-run
```

`--request-timeout` is in milliseconds and defaults to `600000`. This controls the HTTP request to the deployed site; the Codex shell/tool timeout must also be set high enough.

## Video Workflow

Use `scripts/newapi-video.mjs` for every video call. It creates tasks with:

- `POST /v1/video/generations`
- `GET /v1/video/generations/{task_id}`

The script keeps billable/render duration at the top level with `--duration` or `--seconds`. Channel-specific extension fields go into `metadata`.

```bash
node scripts/newapi-video.mjs \
  --mode i2v \
  --model doubao-seedance-1-0-lite-i2v \
  --prompt "Camera slowly pushes in." \
  --image-url https://example.com/first-frame.png \
  --duration 5
```

Text-to-video:

```bash
node scripts/newapi-video.mjs \
  --mode t2v \
  --model doubao-seedance-2-0-fast-260128 \
  --prompt "FPV drone aerial shot over a misty mountain valley." \
  --duration 4 \
  --resolution 720P \
  --ratio 16:9 \
  --generate-audio false
```

First-frame image-to-video:

```bash
node scripts/newapi-video.mjs \
  --mode i2v \
  --model doubao-seedance-2-0-fast-260128 \
  --prompt "Sea breeze across her face, hair and skirt moving gently." \
  --first-frame-file ./inputs/person-by-the-sea.jpg \
  --duration 5 \
  --resolution 720P \
  --ratio 9:16 \
  --generate-audio false
```

First/last-frame video:

```bash
node scripts/newapi-video.mjs \
  --mode i2v \
  --model doubao-seedance-2-0-fast-260128 \
  --prompt "Fly from the snowy mountain opening to the coastal sunset ending." \
  --first-frame-file ./inputs/start.jpg \
  --last-frame-file ./inputs/end.jpg \
  --duration 4 \
  --resolution 720P
```

Reference media video:

```bash
node scripts/newapi-video.mjs \
  --mode r2v \
  --model doubao-seedance-2-0-fast-260128 \
  --prompt "Keep the landscape style consistent while flying forward." \
  --reference-image-files "./inputs/ref-a.jpg;./inputs/ref-b.jpg;./inputs/ref-c.jpg" \
  --duration 4 \
  --resolution 720P
```

Poll an existing task:

```bash
node scripts/newapi-video.mjs \
  --task-id task_123 \
  --out outputs/result.mp4
```

Supported common flags: `--model`, `--prompt`, `--prompt-file`, `--mode`, `--duration`, `--seconds`, `--resolution`, `--ratio`, `--width`, `--height`, `--fps`, `--seed`, `--n`, `--response-format`, `--user`, `--poll-interval`, `--timeout`, `--no-poll`, `--task-id`, `--base-url`, `--dry-run`, `--request-out`, `--create-response-out`, `--response-out`.

Media input flags:

- URL flags: `--image-url`, `--first-frame-url`, `--last-frame-url`, `--reference-image-url`.
- Local file flags: `--image-file`, `--first-frame-file`, `--last-frame-file`, `--reference-image-file`.
- Multiple local references: `--reference-image-files "a.jpg;b.jpg;c.jpg"`.
- Video/audio flags: `--video-url`, `--video-file`, `--reference-video-url`, `--reference-video-file`, `--audio-url`, `--audio-file`, `--reference-audio-url`, `--reference-audio-file`.

Channel extension helpers:

- `--provider doubao|kling|happyhorse` only shapes gateway `metadata`; it is not a separate API path.
- `--extra-json`, `--metadata-json`, `--input-json`, `--parameters-json`, `--content-json`, and `--media-json` pass additional gateway-compatible fields.
- `--generate-audio`, `--return-last-frame`, `--service-tier`, `--execution-expires-after`, `--frames`, `--camera-fixed`, `--watermark`, `--draft`, `--draft-task-id`, `--web-search`, `--safety-identifier`, and `--priority` are written as channel extension metadata when provided.

## Inference Workflow

Use the deployed site's Responses-compatible endpoint only for helper text such as prompt drafts:

```bash
node scripts/infer.mjs \
  --input "Write a concise product video prompt for a coffee cup reveal shot." \
  --model gpt-5.5
```

## References

Read `references/newapi-video-api.md` before changing video request behavior. Keep examples customer-facing: they should call the deployed site, not upstream vendors.

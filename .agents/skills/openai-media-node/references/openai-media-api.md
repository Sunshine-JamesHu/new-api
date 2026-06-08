# OpenAI Media API Notes

Use official OpenAI docs as the source of truth:

- Image generation guide: https://developers.openai.com/api/docs/guides/image-generation.md
- Video generation guide: https://developers.openai.com/api/docs/guides/video-generation.md
- Node SDK: https://github.com/openai/openai-node

## Images

- Prefer the Image API for one-shot image generation.
- Default direct image model: `gpt-image-2`.
- Node SDK pattern:

```js
const result = await openai.images.generate({
  model: "gpt-image-2",
  prompt,
});
```

- `gpt-image-2` size supports values such as `1024x1024`, `1536x1024`, `1024x1536`, `2048x2048`, `2048x1152`, `3840x2160`, `2160x3840`, and `auto`, subject to official constraints.
- `quality` supports `low`, `medium`, `high`, and `auto`.
- `output_format` can be `png`, `jpeg`, or `webp`.
- `output_compression` applies to JPEG/WebP.
- `background: "transparent"` is not supported by `gpt-image-2`; use `auto` or omit it.

## Videos

- The Videos API is asynchronous: create a job, poll until terminal status, then download content.
- Default models:
  - `sora-2`: fast iteration.
  - `sora-2-pro`: higher quality and 1080p exports.
- Official parameters for create jobs include `model`, `prompt`, `size`, `seconds`, `input_reference`, and `characters`.
- `sora-2` and `sora-2-pro` support 16- and 20-second generations; shorter values such as 8 seconds are common in examples.
- Use `sora-2-pro` for `1920x1080` or `1080x1920`.
- Common sizes from the docs include `1280x720`, `1920x1080`, and `1080x1920`.
- For image references:
  - Use `input_reference`.
  - The image should match the output video resolution.
  - Supported formats are JPEG, PNG, and WebP.
- Download completed videos with `openai.videos.downloadContent(video.id)`.

## Safety And Failures

- Video prompts and reference media must be suitable for under-18 audiences.
- Copyrighted characters/music and real people/public figures are restricted.
- Human faces in input images are currently rejected for video references.
- For media API failures, log `request_id`, status, code, and moderation details when present.

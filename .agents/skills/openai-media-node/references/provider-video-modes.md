# Provider Video Modes

Use these notes when building scripts for Doubao, Kling, and HappyHorse video APIs. Verify against the official provider docs before changing payload shapes.

User-provided docs:

- Doubao Seedance: https://www.volcengine.com/docs/82379/1520757?lang=zh
- HappyHorse on Bailian:
  - https://bailian.console.aliyun.com/cn-beijing?tab=api#/api/?type=model&url=3029820
  - https://bailian.console.aliyun.com/cn-beijing?tab=api#/api/?type=model&url=3029821
  - https://bailian.console.aliyun.com/cn-beijing?tab=api#/api/?type=model&url=3030778
- Kling on Bailian:
  - https://bailian.console.aliyun.com/cn-beijing?tab=api#/api/?type=model&url=3026701

## Modes

Always classify the request before building payloads:

- `t2v`: text-to-video. Prompt-only request.
- `i2v`: image-to-video. Uses a first frame or image reference.
- `r2v`: reference-to-video. Uses one or more reference images/videos for identity/style/object consistency.
- `edit`: video editing. Uses an existing video plus edit prompt; may also include reference images.

## NewApi Key Policy

In this skill, all normal generation calls go through the user's NewApi deployment:

- Video and inference use `VIDEO_API_KEY`.
- Image generation uses `IMAGE_API_KEY`.
- Do not add provider-specific keys to `.env.example` unless the user explicitly asks to call provider APIs directly.

## Doubao Seedance

Endpoint shape used by this project:

- Create: `POST {base_url}/api/v3/contents/generations/tasks`
- Fetch: `GET {base_url}/api/v3/contents/generations/tasks/{task_id}`
- Auth through NewApi: `Authorization: Bearer <VIDEO_API_KEY>`.
- Direct provider auth exists but is not part of the default skill workflow.

Payload shape:

```json
{
  "model": "doubao-seedance-...",
  "content": [
    {"type": "image_url", "image_url": {"url": "https://..."}},
    {"type": "video_url", "video_url": {"url": "https://..."}},
    {"type": "text", "text": "prompt"}
  ],
  "duration": 5,
  "resolution": "1080P",
  "ratio": "16:9"
}
```

Mode mapping:

- `t2v`: include text content only.
- `i2v`: include one `image_url` item plus text.
- `r2v`: include one or more `image_url` or `video_url` items plus text. Follow the official model's limits.
- `edit`: include one `video_url` item plus text; add image references if the official model supports them.

Keep top-level `duration` or `seconds` as the trusted duration in gateway code. Do not let nested metadata override billing duration.

## HappyHorse

Endpoint shape used by this project:

- Create: `POST {base_url}/api/v1/services/aigc/video-generation/video-synthesis`
- Fetch: `GET {base_url}/api/v1/tasks/{task_id}`
- Auth through NewApi: `Authorization: Bearer <VIDEO_API_KEY>`.
- Direct provider auth exists but is not part of the default skill workflow.
- Headers: `X-DashScope-Async: enable`, `X-DashScope-OssResourceResolve: enable`

Payload shape:

```json
{
  "model": "happyhorse-1.0-r2v",
  "input": {
    "prompt": "prompt",
    "media": [
      {"type": "reference_image", "url": "https://..."}
    ]
  },
  "parameters": {
    "duration": 5,
    "resolution": "1080P",
    "watermark": false
  }
}
```

Mode/model mapping:

- `happyhorse-1.0-t2v`: text-to-video, action equivalent `textGenerate`.
- `happyhorse-1.0-i2v`: image-to-video, first image as `first_frame`.
- `happyhorse-1.0-r2v`: reference-to-video, media entries as `reference_image`.
- `happyhorse-1.0-video-edit`: video edit, source media as `video` and additional refs as `reference_image`.

## Kling

Direct Kling endpoint shape used by this project:

- Text-to-video: `POST {base_url}/v1/videos/text2video`
- Image-to-video: `POST {base_url}/v1/videos/image2video`
- Fetch text task: `GET {base_url}/v1/videos/text2video/{task_id}`
- Fetch image task: `GET {base_url}/v1/videos/image2video/{task_id}`
- Auth through NewApi: `Authorization: Bearer <VIDEO_API_KEY>`.
- Direct Kling auth uses JWT from access/secret keys, but direct provider auth is not part of the default skill workflow.

Bailian Kling models use the Ali/DashScope video synthesis format with model names such as `kling/kling-v3-video-generation`.

Direct payload shape:

```json
{
  "model_name": "kling-v2-master",
  "prompt": "prompt",
  "image": "https://...",
  "image_tail": "https://...",
  "duration": "5",
  "mode": "std",
  "aspect_ratio": "16:9"
}
```

Mode mapping:

- `t2v`: call `text2video`, omit `image` and `image_tail`.
- `i2v`: call `image2video`, set `image`; optional `image_tail` for end frame.
- `r2v`: for Bailian Kling, use `input.media` with `reference_image` or provider-supported media types.
- `edit`: use official Kling/Bailian edit model fields when available; pass through `--extra-json` rather than guessing.

## Script Extension Rule

If the docs expose new mode-specific parameters, prefer adding them through `--extra-json`, `--input-json`, `--parameters-json`, `--content-json`, or `--media-json` first. Add named flags only for fields used repeatedly across tasks.

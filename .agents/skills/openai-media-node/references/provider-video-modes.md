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

Official Ark create-task docs cover Seedance 2.0, Seedance 2.0 fast, Seedance 1.5 pro, Seedance 1.0 pro, and Seedance 1.0 pro fast. When using this skill through NewApi, keep official provider fields in `metadata`; the gateway converts them to the upstream body.

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
  "resolution": "720p",
  "ratio": "16:9",
  "generate_audio": false,
  "return_last_frame": false,
  "watermark": false
}
```

NewApi wrapper payload shape:

```json
{
  "model": "doubao-seedance-2-0-fast-260128",
  "prompt": "prompt",
  "duration": 5,
  "resolution": "720P",
  "metadata": {
    "content": [
      {"type": "image_url", "image_url": {"url": "data:image/jpeg;base64,..."}, "role": "first_frame"},
      {"type": "text", "text": "prompt"}
    ],
    "ratio": "9:16",
    "generate_audio": false
  }
}
```

The gateway trusts top-level `duration` / `seconds` for billing and upstream duration. Do not rely on `metadata.duration`.

Mode mapping:

- `t2v`: include text content only.
- `i2v`: include one `image_url` item with `role: "first_frame"` plus text. The role may be omitted for single first-frame mode, but the script sends it explicitly.
- First/last-frame I2V: include exactly two `image_url` items with `role: "first_frame"` and `role: "last_frame"`. Prefer this over reference mode when exact first/end frame matching matters.
- `r2v`: Seedance 2.0 reference mode accepts reference images/videos/audios plus optional text. Use `role: "reference_image"`, `role: "reference_video"`, and `role: "reference_audio"`.
- `edit`: include a `video_url` item with `role: "reference_video"` plus text; add image/audio references only when the selected model supports them.
- Draft finalization: Seedance 1.5 pro supports `content` item `{ "type": "draft_task", "draft_task": { "id": "..." } }`.

Official content and file constraints:

- Image URL values may be public URLs, data URLs, or `asset://...` IDs. Formats include JPEG, PNG, WebP, BMP, TIFF, GIF; Seedance 1.5 pro and 2.0 also support HEIC/HEIF. Single image must be under 30 MB, request body under 64 MB.
- Single first-frame mode uses 1 image. First/last-frame mode uses 2 images. Seedance 2.0 reference image mode uses 1-9 images.
- Reference video is Seedance 2.0 only. Formats: MP4 or MOV, H.264/H.265 video with AAC/MP3 audio. Up to 3 reference videos, each 2-15 seconds, total reference video duration up to 15 seconds, each under 50 MB.
- Reference audio is Seedance 2.0 only. Formats: WAV or MP3. Up to 3 audios, each 2-15 seconds, total audio duration up to 15 seconds, each under 15 MB. Audio cannot be the only reference input; include at least one image or video.
- Seedance 2.0 does not support direct upload of many real-person face references. Upstream may return `InputImageSensitiveContentDetected.PrivacyInformation` or policy errors; pass those failures to the caller.

Model capability notes:

- Seedance 2.0 / 2.0 fast: text-to-video, first-frame I2V, first/last-frame I2V, multimodal reference generation, optional audio generation, web search, priority.
- Seedance 1.5 pro: text-to-video, first-frame I2V, first/last-frame I2V, audio generation, draft mode.
- Seedance 1.0 pro: text-to-video, first-frame I2V, first/last-frame I2V.
- Seedance 1.0 pro fast: text-to-video and first-frame I2V.

Official parameter notes:

- `resolution`: lowercase upstream values `480p`, `720p`, `1080p`. Seedance 2.0 / 1.5 default to `720p`; Seedance 1.0 pro/pro-fast default to `1080p`. Seedance 2.0 fast does not support `1080p`.
- `ratio`: `16:9`, `4:3`, `1:1`, `3:4`, `9:16`, `21:9`, or `adaptive`. Seedance 2.0 / 1.5 default to `adaptive`; other text-to-video models default to `16:9`, while image-to-video defaults to `adaptive`.
- 720p 9:16 maps to 720x1280 for Seedance 2.0 / 1.5 and 704x1248 for Seedance 1.0. 1080p 9:16 maps to 1080x1920 for Seedance 2.0 / 1.5 and 1088x1920 for Seedance 1.0.
- `duration`: integer seconds, default 5. Seedance 1.0 pro/pro-fast supports 2-12 seconds. Seedance 1.5 pro supports 4-12 or `-1`. Seedance 2.0 supports 4-15 or `-1`.
- `frames`: official alternative to duration, with higher provider priority than duration, but Seedance 2.0 / 1.5 currently do not support it.
- `generate_audio`: default true upstream for supported models; the NewApi gateway does not support this top-level field, so send it through `metadata` or `--generate-audio true|false`.
- `return_last_frame`: default false. When true, the fetch API can return a PNG last frame for chaining videos.
- `service_tier`: `default` or `flex`; Seedance 2.0 only supports online mode and does not support setting this parameter. Flex does not support priority.
- `execution_expires_after`: task expiration in seconds, range 3600-259200, default 172800.
- `draft`: Seedance 1.5 pro only. Draft mode uses 480p and does not support return-last-frame or flex.
- `tools`: Seedance 2.0 supports `[{ "type": "web_search" }]`.
- `safety_identifier`: stable hashed end-user identifier, max 64 characters.
- `priority`: Seedance 2.0 only, 0-9, FIFO within equal priorities.
- `seed`: `-1` or integer up to `2^32-1`.
- `camera_fixed`: default false; unsupported for reference image scenes and Seedance 2.0.
- `watermark`: default false.

The script normalizes `720P`/`1080P` style input to lowercase inside Doubao metadata before sending upstream. Audio generation is provider metadata only: use `metadata.generate_audio`, or the script flag `--generate-audio true|false`.

For new-api per-second billing, Doubao emits both `seconds` and a normalized resolution tier key. Configure `billing_setting.per_second_multipliers` with lowercase keys such as `resolution-720p` and `resolution-1080p`.

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

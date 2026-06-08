# new-api Video API

Official docs:

- Create video generation: https://www.newapi.ai/zh/docs/api/ai-model/videos/createvideogeneration
- Get video generation: https://www.newapi.ai/zh/docs/api/ai-model/videos/getvideogeneration

## Create

Use:

```http
POST /v1/video/generations
Authorization: Bearer <token>
Content-Type: application/json
```

Request fields documented by newapi:

- `model`: model or style ID.
- `prompt`: text prompt.
- `image`: image input URL or base64.
- `duration`: video duration in seconds.
- `width`, `height`: output dimensions.
- `fps`: frame rate.
- `seed`: random seed.
- `n`: number of videos.
- `response_format`: response format.
- `user`: user identifier.
- `metadata`: provider-specific extension parameters.

Response:

```json
{
  "task_id": "abcd1234efgh",
  "status": "queued"
}
```

## Get

Use:

```http
GET /v1/video/generations/{task_id}
Authorization: Bearer <token>
```

Task statuses:

- `queued`
- `in_progress`
- `completed`
- `failed`

Completed response includes `url`, `format`, `metadata`, and optional `error`.

## Billing-Safe Duration Rule

This repo was updated so Doubao and Kling follow the same per-second billing logic as Jimeng and HappyHorse:

- Trust only top-level `duration` first.
- Then top-level `seconds`, rounded up.
- Default to 5 seconds.
- Do not let `metadata.duration` or `metadata.parameters.duration` override billing duration.

For scripts, always send the intended billable/render duration at the top level. Provider-specific duration fields may be mirrored inside metadata only when the official provider requires them, but they must not be the only duration source.

## Mode Mapping Through new-api

Use `metadata` for provider-specific details:

- Doubao:
  - `metadata.content` may contain `text`, `image_url`, and `video_url` items.
  - Keep top-level `prompt`, `image`, and `duration` populated for compatibility.
- HappyHorse:
  - `metadata.input.media` carries `first_frame`, `reference_image`, or `video`.
  - `metadata.parameters` carries `resolution`, `size`, `watermark`, `audio`, `seed`, etc.
- Kling:
  - Direct Kling uses top-level `image` for I2V.
  - `metadata.image_tail`, `metadata.mode`, `metadata.aspect_ratio`, `metadata.camera_control`, masks, and other official fields pass through.
  - Bailian Kling models can use `metadata.input.media` / `metadata.parameters` like Ali/DashScope video synthesis.

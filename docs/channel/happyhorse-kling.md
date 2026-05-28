# 快乐马与可灵视频调用说明

本文说明如何在 new-api 中调用快乐马（HappyHorse）与可灵（Kling）视频生成通道，包含渠道配置、提交任务、查询任务、参数透传和提示词建议。

## 一、能力与入口

### 快乐马（HappyHorse）

快乐马使用统一视频任务接口：

- 提交任务：`POST /v1/video/generations`
- 查询任务：`GET /v1/video/generations/{task_id}`
- OpenAI 兼容提交：`POST /v1/videos`
- OpenAI 兼容查询：`GET /v1/videos/{task_id}`

支持模型：

| 模型 | 用途 | 常用输入 |
| --- | --- | --- |
| `happyhorse-1.0-t2v` | 文生视频 | `prompt` |
| `happyhorse-1.0-i2v` | 图生视频/首帧生视频 | `prompt` + `images` 或 `image` |
| `happyhorse-1.0-r2v` | 参考图生视频 | `prompt` + 多张 `images` |
| `happyhorse-1.0-video-edit` | 视频编辑 | `prompt` + 视频 URL，可附参考图 |

默认上游地址：`https://dashscope.aliyuncs.com`

### 可灵（Kling）

可灵支持两种调用方式。

方式一，使用统一视频任务接口：

- 提交任务：`POST /v1/video/generations`
- 查询任务：`GET /v1/video/generations/{task_id}`

方式二，使用可灵官方风格路径：

- 文生视频：`POST /kling/v1/videos/text2video`
- 文生视频查询：`GET /kling/v1/videos/text2video/{task_id}`
- 图生视频：`POST /kling/v1/videos/image2video`
- 图生视频查询：`GET /kling/v1/videos/image2video/{task_id}`

支持模型：

| 模型 | 用途 |
| --- | --- |
| `kling-v1` | 可灵基础视频模型 |
| `kling-v1-6` | 可灵 1.6 |
| `kling-v2-master` | 可灵 2 Master |

默认上游地址：`https://api.klingai.com`

## 二、渠道配置

### 快乐马渠道

在渠道管理中新增渠道：

- 类型：`HappyHorse`
- 模型：`happyhorse-1.0-t2v,happyhorse-1.0-i2v,happyhorse-1.0-r2v,happyhorse-1.0-video-edit`
- 密钥：填写 DashScope API Key
- 代理地址：默认 `https://dashscope.aliyuncs.com`

快乐马上游请求会自动添加：

- `Authorization: Bearer {API Key}`
- `X-DashScope-Async: enable`
- `X-DashScope-OssResourceResolve: enable`

因此输入中可以使用可被 DashScope 解析的 OSS 或公网资源 URL。

### 可灵渠道

在渠道管理中新增渠道：

- 类型：`Kling`
- 模型：`kling-v1,kling-v1-6,kling-v2-master`
- 密钥：填写 `accessKey|secretKey`
- 代理地址：默认 `https://api.klingai.com`

可灵通道会使用 `accessKey|secretKey` 自动生成 JWT。若上游是另一个 new-api 中继，密钥也可以使用 `sk-...` 格式，此时请求会转发到 `/kling/v1/...` 路径。

## 三、快乐马调用示例

所有示例中的 `$BASE_URL` 为当前 new-api 服务地址，`$NEW_API_KEY` 为用户令牌。

### 1. 文生视频

```bash
curl "$BASE_URL/v1/video/generations" \
  -H "Authorization: Bearer $NEW_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "happyhorse-1.0-t2v",
    "prompt": "一匹白马在清晨草原上慢跑，低角度跟拍，阳光从侧后方穿过鬃毛，草叶上有露水反光，真实电影质感",
    "duration": 5,
    "size": "1080p"
  }'
```

快乐马默认参数：

- 未传 `duration` 时默认 `5`
- 未传 `size` 时默认 `resolution=1080P`
- `happyhorse-1.0-t2v` 与 `happyhorse-1.0-r2v` 默认 `ratio=16:9`
- 默认 `prompt_extend=true`
- 默认 `watermark=false`

### 2. 图生视频

```bash
curl "$BASE_URL/v1/video/generations" \
  -H "Authorization: Bearer $NEW_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "happyhorse-1.0-i2v",
    "prompt": "保持首帧构图和角色朝向，人物轻轻抬头并眨眼，衣摆被微风带动，镜头缓慢前推",
    "images": ["https://example.com/first-frame.png"],
    "duration": 5,
    "size": "720p"
  }'
```

对 `happyhorse-1.0-i2v`，`images` 中第一张图会被转换为上游 `media`：

```json
[
  {
    "type": "first_frame",
    "url": "https://example.com/first-frame.png"
  }
]
```

也可以使用兼容字段 `image`：

```json
{
  "model": "happyhorse-1.0-i2v",
  "prompt": "保持首帧画面，镜头轻微推进",
  "image": "https://example.com/first-frame.png"
}
```

### 3. 参考图生视频

```bash
curl "$BASE_URL/v1/video/generations" \
  -H "Authorization: Bearer $NEW_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "happyhorse-1.0-r2v",
    "prompt": "参考图中的人物身份、服装和材质保持一致，在同一光线氛围下转身看向镜头，背景轻微虚化",
    "images": [
      "https://example.com/character.png",
      "https://example.com/style-reference.png"
    ],
    "duration": 6,
    "size": "1080p"
  }'
```

对 `happyhorse-1.0-r2v`，每张 `images` 都会转换为 `reference_image`。

### 4. 视频编辑

```bash
curl "$BASE_URL/v1/video/generations" \
  -H "Authorization: Bearer $NEW_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "happyhorse-1.0-video-edit",
    "prompt": "保持原视频主体、镜头路径和动作节奏，只把背景改成夜晚霓虹街道，人物服装和脸部不变",
    "images": [
      "https://example.com/source.mp4",
      "https://example.com/neon-reference.png"
    ],
    "duration": 5,
    "size": "1080p"
  }'
```

对 `happyhorse-1.0-video-edit`：

- URL 看起来像 `.mp4`、`.mov` 或 `data:video/` 时，会作为 `video`
- 其他 URL 会作为 `reference_image`

### 5. 高级参数透传

快乐马支持通过 `metadata`、`input`、`parameters` 透传上游字段。

```json
{
  "model": "happyhorse-1.0-t2v",
  "prompt": "雨夜街头，黑色轿车驶过积水路面，低角度跟拍，水花和车灯反光清晰",
  "metadata": {
    "negative_prompt": "低清晰度，画面抖动过强，主体变形",
    "seed": 12345,
    "ratio": "16:9",
    "resolution": "1080P",
    "prompt_extend": true,
    "watermark": false
  }
}
```

等价的嵌套写法：

```json
{
  "model": "happyhorse-1.0-r2v",
  "prompt": "保持参考人物身份，缓慢转身",
  "metadata": {
    "input": {
      "media": [
        {
          "type": "reference_image",
          "url": "https://example.com/ref.png"
        }
      ]
    },
    "parameters": {
      "duration": 5,
      "resolution": "1080P",
      "watermark": false
    }
  }
}
```

注意：`metadata`、`input`、`parameters` 不能修改最终上游模型名。如果其中包含与请求模型不一致的 `model`，请求会失败。

## 四、可灵调用示例

### 1. 统一接口文生视频

```bash
curl "$BASE_URL/v1/video/generations" \
  -H "Authorization: Bearer $NEW_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "kling-v1",
    "prompt": "一只橘猫在花园里弹钢琴，镜头缓慢推进，阳光柔和",
    "duration": 5,
    "size": "1920x1080"
  }'
```

`size` 会映射为可灵 `aspect_ratio`：

| `size` | `aspect_ratio` |
| --- | --- |
| `1024x1024`、`512x512` | `1:1` |
| `1280x720`、`1920x1080` | `16:9` |
| `720x1280`、`1080x1920` | `9:16` |
| 其他值 | `1:1` |

### 2. 统一接口图生视频

```bash
curl "$BASE_URL/v1/video/generations" \
  -H "Authorization: Bearer $NEW_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "kling-v2-master",
    "image": "https://example.com/start.png",
    "prompt": "保持图片主体和构图，镜头缓慢右移，人物自然眨眼，背景光线轻微变化",
    "duration": 5,
    "size": "1280x720"
  }'
```

### 3. 可灵官方风格文生视频

```bash
curl "$BASE_URL/kling/v1/videos/text2video" \
  -H "Authorization: Bearer $NEW_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model_name": "kling-v1",
    "prompt": "A cat playing piano in the garden, soft sunlight, slow camera push-in",
    "negative_prompt": "blurry, low quality",
    "cfg_scale": 0.7,
    "mode": "std",
    "aspect_ratio": "16:9",
    "duration": "5",
    "camera_control": {
      "type": "simple",
      "config": {
        "horizontal": 0,
        "vertical": 0,
        "pan": 0,
        "tilt": 0,
        "roll": 0,
        "zoom": 1.5
      }
    }
  }'
```

### 4. 可灵官方风格图生视频

```bash
curl "$BASE_URL/kling/v1/videos/image2video" \
  -H "Authorization: Bearer $NEW_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model_name": "kling-v2-master",
    "image": "https://example.com/start.png",
    "prompt": "Keep the first frame composition, the subject turns slightly toward camera, natural blinking, slow dolly in",
    "negative_prompt": "blur, distorted face, extra limbs",
    "mode": "std",
    "duration": "5",
    "aspect_ratio": "16:9"
  }'
```

官方风格路径会把原始请求整体放入 `metadata` 后进入统一任务流，所以 `negative_prompt`、`cfg_scale`、`camera_control`、`callback_url`、`external_task_id` 等字段可以按可灵格式传入。

## 五、查询任务与读取结果

提交任务后会返回一个公开任务 ID，后续用该 ID 查询。

示例响应：

```json
{
  "id": "task_xxxxxxxxxxxxx",
  "task_id": "task_xxxxxxxxxxxxx",
  "object": "video",
  "model": "happyhorse-1.0-t2v",
  "status": "queued",
  "progress": 0,
  "created_at": 1760000000
}
```

查询统一接口：

```bash
curl "$BASE_URL/v1/video/generations/task_xxxxxxxxxxxxx" \
  -H "Authorization: Bearer $NEW_API_KEY"
```

查询 OpenAI 兼容接口：

```bash
curl "$BASE_URL/v1/videos/task_xxxxxxxxxxxxx" \
  -H "Authorization: Bearer $NEW_API_KEY"
```

可灵官方风格查询：

```bash
curl "$BASE_URL/kling/v1/videos/text2video/task_xxxxxxxxxxxxx" \
  -H "Authorization: Bearer $NEW_API_KEY"
```

或：

```bash
curl "$BASE_URL/kling/v1/videos/image2video/task_xxxxxxxxxxxxx" \
  -H "Authorization: Bearer $NEW_API_KEY"
```

成功完成后，OpenAI 兼容响应会在 `metadata.url` 中返回视频地址：

```json
{
  "id": "task_xxxxxxxxxxxxx",
  "object": "video",
  "model": "happyhorse-1.0-t2v",
  "status": "completed",
  "progress": 100,
  "metadata": {
    "url": "https://example.com/result.mp4"
  }
}
```

## 六、参数说明

### 通用请求字段

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `model` | string | 模型名，统一接口必填 |
| `prompt` | string | 文本提示词 |
| `image` | string | 单图输入，常用于图生视频 |
| `images` | string[] | 多媒体输入，快乐马会按模型转换媒体类型 |
| `size` | string | 分辨率或比例提示。快乐马可传 `720p`、`1080p`；可灵常传 `1920x1080`、`1080x1920` |
| `duration` | number/string | 视频时长秒数，默认 5。小数会向上取整 |
| `seconds` | string | 兼容时长字段，优先级高于普通默认值 |
| `mode` | string | 上游模式。可灵默认 `std` |
| `metadata` | object/string | 上游扩展参数 |
| `input` | object | 快乐马上游 `input` 覆盖参数 |
| `parameters` | object | 快乐马上游 `parameters` 覆盖参数 |

### 快乐马常用透传字段

| 字段 | 放置位置 | 说明 |
| --- | --- | --- |
| `negative_prompt` | `metadata` 或 `input` | 负向提示词 |
| `media` | `metadata.input`、`metadata.media` 或 `input.media` | 上游媒体列表 |
| `resolution` | `metadata`、`metadata.parameters` 或 `parameters` | 如 `720P`、`1080P` |
| `ratio` | 同上 | 如 `16:9`、`9:16` |
| `prompt_extend` | 同上 | 是否开启提示词扩展 |
| `watermark` | 同上 | 是否加水印 |
| `seed` | 同上 | 随机种子，显式传 `0` 会保留 |
| `audio` | 同上 | 是否启用音频相关能力，取决于上游模型支持 |
| `audio_setting` | 同上 | 音频参数，取决于上游模型支持 |

### 可灵常用透传字段

| 字段 | 说明 |
| --- | --- |
| `model_name` | 官方风格模型字段，也兼容 `model` |
| `negative_prompt` | 负向提示词 |
| `cfg_scale` | 提示词相关性 |
| `mode` | 生成模式 |
| `duration` | 字符串或数字均可，最终转为整数字符串 |
| `aspect_ratio` | 画幅比例 |
| `camera_control` | 镜头控制 |
| `callback_url` | 回调地址 |
| `external_task_id` | 外部任务 ID |
| `image_tail` | 尾帧图，按上游能力使用 |
| `static_mask`、`dynamic_masks` | 局部控制相关字段，按上游能力使用 |

## 七、提示词建议

### 快乐马更适合的写法

快乐马对光线、材质、慢镜头、情绪和明确动作通常更友好。建议：

- 用可见结果描述画面：`侧后方阳光穿过发丝`、`玻璃杯边缘有高光`、`浅景深背景虚化`
- 把动作拆成清楚顺序：起始姿态、运动方向、接触点、结果变化
- I2V/R2V/V2V 要先锁定来源：`保持首帧构图`、`保持人物身份和服装`、`保持原视频镜头路径`
- 避免把太多镜头塞进一个 prompt；必要时用短句说明 2 到 3 个关键阶段
- 对不想要的内容，尽量转换成正向画面目标。例如把“不要静止”写成“自然呼吸、眨眼、衣物轻微摆动”

### 快乐马示例提示词

文生视频：

```text
雨后的城市天台，一名穿黑色风衣的女孩站在栏杆旁，侧后方冷色霓虹光勾出头发边缘。她缓慢转头看向镜头，眼神平静，风吹动衣摆和发丝。镜头低速前推，背景高楼灯光形成柔和散景，真实电影质感。
```

图生视频：

```text
保持首帧构图、人物身份、服装和镜头角度。人物从当前姿态开始轻轻抬头，眨眼一次，右手整理衣领，衣料随动作产生细小褶皱。镜头缓慢前推，背景保持不变，仅有轻微景深变化。
```

视频编辑：

```text
保持原视频主体身份、动作节奏、镜头路径和画面构图，只将背景替换为雨夜霓虹街道。人物脸部、服装颜色和手部动作不改变，地面增加积水反光，车灯和招牌光在背景中轻微闪烁。
```

### 可灵更适合的写法

可灵调用时建议充分利用官方参数：

- 文生视频优先明确 `aspect_ratio`、`duration`、`mode`
- 需要镜头运动时使用 `camera_control`，不要在 prompt 里堆叠多个冲突镜头
- 图生视频 prompt 先写“保持图片主体/构图”，再写运动
- `negative_prompt` 保持短而具体，如 `blur, distorted face, extra limbs`

## 八、计费与时长

如果模型计费模式配置为 `per_second`：

- 快乐马和可灵都会按 `duration`/`seconds` 估算秒数
- 未传时长默认按 5 秒估算
- 快乐马还会带上分辨率倍率，例如 `resolution-1080P`
- `duration` 同时出现在多个位置且值不一致时，快乐马会返回 `duration mismatch`

快乐马常用倍率示例：

```json
{
  "happyhorse-1.0-t2v": {
    "resolution-720P": 1,
    "resolution-1080P": 1.777778
  }
}
```

## 九、常见问题

### 1. 快乐马图生视频没有识别图片？

确认使用了 `happyhorse-1.0-i2v`，并传入 `image` 或 `images`。如果已经自己传 `input.media`，系统不会再自动从 `image/images` 生成媒体列表。

### 2. 快乐马视频编辑把参考图当成视频怎么办？

自动识别视频依赖 URL 特征：`.mp4`、`.mov` 或 `data:video/`。如果 URL 不明显，建议直接传 `input.media`：

```json
{
  "model": "happyhorse-1.0-video-edit",
  "prompt": "保持原视频动作，只修改背景",
  "input": {
    "media": [
      {
        "type": "video",
        "url": "https://example.com/source"
      },
      {
        "type": "reference_image",
        "url": "https://example.com/style.png"
      }
    ]
  }
}
```

### 3. 可灵官方路径和统一接口应该选哪个？

如果客户端已经按可灵官方 API 写好，使用 `/kling/v1/...` 更省改造。如果是新接入或希望多视频供应商统一，使用 `/v1/video/generations`。

### 4. 查询结果没有视频 URL？

任务还没完成时不会有 URL。等待状态变为 `completed` 或 `succeed` 后再读取结果；OpenAI 兼容响应中一般在 `metadata.url`。

# 视频模型调用说明

本文说明如何在 new-api 中通过统一视频接口调用快乐马（HappyHorse）和即梦（Jimeng）视频模型。当前推荐方式是：外层只放网关需要理解的字段，真实厂商参数全部放进 `metadata`。

## 一、统一入口

提交任务：

```http
POST /v1/videos
```

兼容旧入口：

```http
POST /v1/video/generations
```

查询任务：

```http
GET /v1/videos/{task_id}
GET /v1/video/generations/{task_id}
```

读取视频内容：

```http
GET /v1/videos/{task_id}/content
```

注意：当前任务中心预览只有即梦会走后端 `/content` 代理，其他视频服务商默认使用任务结果中的原始 `metadata.url`。

## 二、通用请求约定

统一视频入口只稳定理解这些外层字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `model` | string | 必填。用户请求模型名，也会参与模型映射。即梦会把最终上游模型名作为 `req_key`。 |
| `duration` | number | 推荐。视频计费可信时长来源，也会被部分通道写入上游参数。 |
| `seconds` | string/number | 兼容字段。仅当外层 `duration` 缺失时用于计费时长。 |
| `resolution` | string | 分辨率计费档位。快乐马只接受 `720P`、`1080P`，大小写会归一化。 |
| `metadata` | object | 真实厂商 API 参数。除网关必须覆盖的计费一致性字段外，按上游格式透传。 |

不要依赖外层 `prompt`、`image`、`images`、`size`、`fps` 组装快乐马或即梦真实请求。调用方应按厂商 API 形状把这些参数放到 `metadata` 中。

计费规则：

- 优先使用外层 `duration`。
- 外层 `duration` 缺失时再使用 `seconds`。
- 两者都缺失时默认 5 秒。
- `metadata.duration`、`metadata.parameters.duration` 和任意深层同名字段不参与计费。
- 快乐马按秒计费时会附加 `seconds` 和 `resolution-720P` / `resolution-1080P` 倍率。
- 即梦按秒计费时只使用 `seconds`；分辨率通常由模型 `req_key` 决定。

## 三、快乐马（HappyHorse）

渠道配置：

- 渠道类型：`HappyHorse`
- 默认上游地址：`https://dashscope.aliyuncs.com`
- 密钥：DashScope API Key
- 模型：`happyhorse-1.0-t2v,happyhorse-1.0-i2v,happyhorse-1.0-r2v,happyhorse-1.0-video-edit`

上游请求会自动添加：

- `Authorization: Bearer {API Key}`
- `X-DashScope-Async: enable`
- `X-DashScope-OssResourceResolve: enable`

### 模型与任务类型

| 模型 | 任务中心类型 | 推荐 metadata |
| --- | --- | --- |
| `happyhorse-1.0-t2v` | 文生视频 | `metadata.input.prompt` |
| `happyhorse-1.0-i2v` | 图生视频 | `metadata.input.prompt` + `metadata.input.media` 首帧图 |
| `happyhorse-1.0-r2v` | 图生视频/参考图生视频 | `metadata.input.prompt` + 多个 `reference_image` |
| `happyhorse-1.0-video-edit` | 图生视频/视频编辑 | `metadata.input.prompt` + `video` / `reference_image` |

快乐马真实上游 body 由 `metadata` 生成：

- `metadata.input` 合并到上游 `input`。
- `metadata.parameters` 合并到上游 `parameters`。
- `metadata` 中未知顶层字段会继续作为上游顶层字段透传。
- 网关会覆盖上游 `parameters.duration` 为外层 `duration` 解析结果。
- 网关会覆盖上游 `parameters.resolution` 为外层 `resolution` 解析结果，默认 `1080P`。
- `metadata.model`、`metadata.input.model`、`metadata.parameters.model` 不允许把模型改成另一个值。

### 快乐马文生视频

```json
{
  "model": "happyhorse-1.0-t2v",
  "duration": 5,
  "resolution": "1080P",
  "metadata": {
    "input": {
      "prompt": "一只毛茸茸的布偶猫在阳光洒满的客厅窗边轻轻跳起，伸出小爪子追逐一只从窗外掠过的小鸟，但不会伤害小鸟。画面温暖治愈，柔和自然光，真实摄影风格，镜头缓慢跟随。"
    },
    "parameters": {
      "aspect_ratio": "16:9",
      "seed": 42
    }
  }
}
```

快乐马最终会向上游发送类似：

```json
{
  "model": "happyhorse-1.0-t2v",
  "input": {
    "prompt": "..."
  },
  "parameters": {
    "aspect_ratio": "16:9",
    "seed": 42,
    "duration": 5,
    "resolution": "1080P"
  }
}
```

### 快乐马图生视频

```json
{
  "model": "happyhorse-1.0-i2v",
  "duration": 5,
  "resolution": "720P",
  "metadata": {
    "input": {
      "prompt": "保持首帧构图和人物身份，人物轻轻抬头，雨水沿玻璃滑落，镜头缓慢后退。",
      "media": [
        {
          "type": "first_frame",
          "url": "https://example.com/first-frame.png"
        }
      ]
    },
    "parameters": {
      "aspect_ratio": "16:9"
    }
  }
}
```

### 快乐马参考图生视频

```json
{
  "model": "happyhorse-1.0-r2v",
  "duration": 5,
  "resolution": "1080P",
  "metadata": {
    "input": {
      "prompt": "保持参考人物身份、服装和材质一致，在同一光线氛围中缓慢转身看向镜头。",
      "media": [
        {
          "type": "reference_image",
          "url": "https://example.com/character.png"
        },
        {
          "type": "reference_image",
          "url": "https://example.com/style.png"
        }
      ]
    },
    "parameters": {
      "aspect_ratio": "16:9",
      "watermark": false
    }
  }
}
```

### 快乐马视频编辑

```json
{
  "model": "happyhorse-1.0-video-edit",
  "duration": 5,
  "resolution": "1080P",
  "metadata": {
    "input": {
      "prompt": "保持原视频主体身份、动作节奏、镜头路径和画面构图，只把背景改成雨夜霓虹街道。",
      "media": [
        {
          "type": "video",
          "url": "https://example.com/source.mp4"
        },
        {
          "type": "reference_image",
          "url": "https://example.com/neon-style.png"
        }
      ]
    },
    "parameters": {
      "aspect_ratio": "16:9"
    }
  }
}
```

## 四、即梦（Jimeng）

渠道配置：

- 渠道类型：`Jimeng`
- 默认上游地址按渠道配置。
- 密钥可使用火山 `access_key|secret_key`，也可使用支持即梦转发的 `sk-...` 中继密钥。

即梦的 `metadata` 就是火山真实 API body 的主体。网关只额外注入或覆盖：

- `req_key`：来自最终上游模型名，即外层 `model` 经过模型映射后的结果。缺失会本地报错。
- `frames`：由外层 `duration` 转换，公式为 `24 * duration + 1`。

即梦查询时必须带保存下来的 `req_key`。提交成功后网关会把最终 `req_key` 保存到任务数据里，轮询任务时用它查询火山结果。

### 即梦文生视频

```json
{
  "model": "jimeng_t2v_v30",
  "duration": 5,
  "metadata": {
    "prompt": "动漫电影感，雨夜写字楼门口，男主抱着纸箱走出玻璃大门，霓虹倒影在地面，低头沉默，镜头缓慢后退。",
    "aspect_ratio": "16:9",
    "seed": -1
  }
}
```

最终上游 body 会包含：

```json
{
  "req_key": "jimeng_t2v_v30",
  "prompt": "...",
  "aspect_ratio": "16:9",
  "seed": -1,
  "frames": 121
}
```

### 即梦图文生视频 Pro

```json
{
  "model": "jimeng_ti2v_v30_pro",
  "duration": 5,
  "metadata": {
    "prompt": "动漫电影感，雨夜写字楼门口，男主抱着纸箱走出玻璃大门，霓虹倒影在地面，低头沉默，镜头缓慢后退。",
    "binary_data_base64": [
      "BASE64_ENCODED_IMAGE"
    ]
  }
}
```

也可以按火山能力使用 `image_urls`：

```json
{
  "model": "jimeng_ti2v_v30_pro",
  "duration": 5,
  "metadata": {
    "prompt": "保持首帧人物和雨夜写字楼场景，男主抱着纸箱低头走出玻璃门，镜头缓慢后退。",
    "image_urls": [
      "https://example.com/first-frame.png"
    ]
  }
}
```

### 即梦模型映射与 req_key

即梦的 `req_key` 不是固定值，而是请求模型映射后的真实上游模型名。例如：

- 外层 `model=jimeng_t2v_v30`，最终 `req_key=jimeng_t2v_v30`。
- 外层 `model=jimeng_ti2v_v30_pro`，最终 `req_key=jimeng_ti2v_v30_pro`。
- 如果渠道配置了模型映射，使用映射后的模型作为 `req_key`。

兼容别名 `jimeng_v30` 时，网关会根据 `metadata` 中图片数量推导：

- 无图片：`jimeng_t2v_v30`
- 1 张图：`jimeng_i2v_first_v30`
- 多张图：`jimeng_i2v_first_tail_v30`
- `jimeng_v30_pro`：`jimeng_ti2v_v30_pro`

## 五、查询与结果

提交成功会返回公开任务 ID：

```json
{
  "id": "task_xxxxxxxxxxxxx",
  "task_id": "task_xxxxxxxxxxxxx",
  "object": "video",
  "model": "jimeng_t2v_v30",
  "status": "queued",
  "progress": 0,
  "created_at": 1780000000
}
```

查询：

```bash
curl "$BASE_URL/v1/videos/task_xxxxxxxxxxxxx" \
  -H "Authorization: Bearer $NEW_API_KEY"
```

完成后 OpenAI 兼容响应会在 `metadata.url` 中返回视频地址：

```json
{
  "id": "task_xxxxxxxxxxxxx",
  "object": "video",
  "status": "completed",
  "progress": 100,
  "metadata": {
    "url": "https://example.com/result.mp4"
  }
}
```

即梦视频 URL 可能存在跨域、防盗链或短期有效期问题。任务中心预览即梦时会使用：

```http
GET /v1/videos/{task_id}/content
```

非即梦视频默认使用 `metadata.url` 原始链接。不要把快乐马等其他服务商强制改成后端代理播放。

## 六、提示词建议

快乐马更适合光线、材质、慢镜头、情绪和清晰动作。建议把动作和镜头写得短而明确：

```text
一只毛茸茸的布偶猫在阳光洒满的客厅窗边轻轻跳起，伸出小爪子追逐一只从窗外掠过的小鸟，但不会伤害小鸟。画面温暖治愈，浅色家居，柔和自然光，镜头缓慢跟随。
```

即梦更接近火山官方参数格式，建议把画面主体、场景、动作、光线、镜头运动写在 `metadata.prompt` 顶层：

```text
动漫电影感，雨夜写字楼门口，男主抱着纸箱走出玻璃大门，霓虹倒影在地面，低头沉默，镜头缓慢后退。
```

## 七、常见问题

### 1. 快乐马没有识别 prompt 或图片？

确认参数放在 `metadata.input` 中，例如 `metadata.input.prompt` 和 `metadata.input.media`。外层 `prompt`、`image`、`images` 不再作为快乐马真实请求组装来源。

### 2. 快乐马扣费秒数不对？

检查外层 `duration`。快乐马真实上游时长和计费都以外层 `duration` 为准；`metadata.parameters.duration` 会被覆盖。

### 3. 快乐马分辨率报错？

外层 `resolution` 只接受 `720P` 或 `1080P`，大小写不敏感。不要使用 `4K`、`1080x1920` 作为外层 `resolution`。

### 4. 即梦 50400 或查询失败？

通常是查询缺少 `req_key` 或提交保存的 `req_key` 不对。当前网关会把最终上游模型名作为 `req_key` 保存并用于查询；外层 `model` 缺失会直接报错。

### 5. 即梦 prompt 不生效？

即梦文生视频需要 `metadata.prompt`。外层 `prompt` 不会作为即梦真实上游 `prompt`。

### 6. 即梦时长如何传？

传外层 `duration`。网关会转换为 `frames = 24 * duration + 1` 后透传给火山。当前不在本地限制 5 秒或 10 秒，不支持的时长由火山上游返回错误。

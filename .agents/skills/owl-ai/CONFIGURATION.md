# OwlAI Skill 配置手册

这份手册给第一次使用 Skill 的用户。配置一次后，可以让 Codex 或 Claude Code 代你调用 OwlAI 的生图和生视频接口。

## 1. 准备 API Key

1. 打开 <https://www.owlai.top> 并登录。
2. 点击左侧 **API 密钥**。
3. 点击右上角 **创建 API 密钥**。
4. 分别创建用途清楚的 Key，例如 `image-test`、`seedance-video`、`happyhorse-video`。
5. 创建后立即复制完整 Key，并保存到本机密码管理器。完整 Key 不要发到聊天、截图或代码仓库。

## 2. 安装 Skill（Windows）

1. 从使用指南下载 `owl-ai.zip`，右键压缩包，选择 **全部解压缩**。
2. 把解压出来的 `owl-ai` 文件夹放到 `C:\Users\你的用户名\.codex\skills\owl-ai`。
3. 打开 PowerShell，执行：

```powershell
cd C:\Users\你的用户名\.codex\skills\owl-ai
npm install
Copy-Item .env.example .env
notepad .env
```

4. 在记事本中填写需要的变量：

```dotenv
NEWAPI_BASE_URL=https://www.owlai.top
IMAGE_API_KEY=sk-你的生图Key
SEEDANCE_API_KEY=sk-你的SeedanceKey
HAPPYHORSE_API_KEY=sk-你的HappyHorseKey
```

只使用生图时只填 `NEWAPI_BASE_URL` 和 `IMAGE_API_KEY`；只使用某个视频模型时填对应的视频 Key。不要在变量值后面加 `/v1`，脚本会自动补上。

## 3. 让 AI 生成图片

重启 Codex 或 Claude Code，然后直接说：

> 使用 owl-ai skill 的 gpt-image-2 生成一张产品图，保存到 outputs/product.png。画面要求：黑色咖啡机放在明亮厨房，横构图。

AI 会调用 `scripts/generate-image.mjs`，图片会保存到 `outputs/`。如果只想先检查请求，不产生费用，可以让 AI 加上 `--dry-run`。

## 4. 让 AI 生成视频

直接说明模型、时长、素材和输出位置，例如：

> 使用 owl-ai skill 的 Seedance 生成 5 秒竖屏视频，首帧是 inputs/start.jpg，镜头缓慢推进，保存到 outputs/result.mp4。

视频接口是异步任务。脚本会自动提交、轮询并下载结果；不要重复提交同一个需求。HappyHorse 的模型名包含 `happyhorse`，Seedance 的模型名包含 `seedance`，脚本会自动选择对应 Key。

## 5. 出错时先看这三项

- `Missing ..._API_KEY`：检查 `.env` 是否存在、变量名是否拼写正确。
- `401` 或 `403`：回到 **API 密钥** 页面，确认 Key 已启用且有对应模型权限。
- `404`：确认 `NEWAPI_BASE_URL=https://www.owlai.top`，不要填写 `/v1` 或具体接口路径。

配置改完后重启 Codex 或 Claude Code，让 Skill 重新读取 `.env`。

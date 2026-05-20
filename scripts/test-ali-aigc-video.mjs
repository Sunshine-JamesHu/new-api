#!/usr/bin/env node

import { readFile } from 'node:fs/promises'
import path from 'node:path'

const DEFAULT_IMAGE_PATH =
  'C:/Users/PayAndHtr/Documents/Tencent Files/690531347/nt_qq/nt_data/Pic/2026-05/Ori/556f321ac95b0e7e495f87ce2608132e.png'

function parseArgs(argv) {
  const args = {
    baseUrl: process.env.NEWAPI_BASE_URL || 'http://localhost:3003',
    apiKey: process.env.NEWAPI_API_KEY || '',
    image: DEFAULT_IMAGE_PATH,
    imageUrl: '',
    duration: 15,
    pollIntervalMs: 10_000,
    maxPolls: 60,
    provider: 'all',
    task: 'all',
    run: false,
    poll: true,
    t2vOnly: false,
    i2vOnly: false,
    useBase64Image: false,
  }

  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i]
    const next = () => argv[++i]
    switch (arg) {
      case '--base-url':
        args.baseUrl = next()
        break
      case '--api-key':
        args.apiKey = next()
        break
      case '--image':
        args.image = next()
        break
      case '--image-url':
        args.imageUrl = next()
        break
      case '--duration':
        args.duration = Number(next())
        break
      case '--poll-interval-ms':
        args.pollIntervalMs = Number(next())
        break
      case '--max-polls':
        args.maxPolls = Number(next())
        break
      case '--provider':
        args.provider = next()
        break
      case '--task':
        args.task = next()
        break
      case '--run':
        args.run = true
        break
      case '--no-poll':
        args.poll = false
        break
      case '--t2v-only':
        args.t2vOnly = true
        break
      case '--i2v-only':
        args.i2vOnly = true
        break
      case '--use-base64-image':
        args.useBase64Image = true
        break
      case '--help':
      case '-h':
        printHelp()
        process.exit(0)
      default:
        throw new Error(`Unknown argument: ${arg}`)
    }
  }

  if (!Number.isFinite(args.duration) || args.duration <= 0) {
    throw new Error('--duration must be a positive number')
  }
  if (args.t2vOnly && args.i2vOnly) {
    throw new Error('--t2v-only and --i2v-only cannot be used together')
  }
  if (!['all', 'happyhorse', 'kling'].includes(args.provider)) {
    throw new Error('--provider must be one of: all, happyhorse, kling')
  }
  if (!['all', 't2v', 'i2v', 'r2v'].includes(args.task)) {
    throw new Error('--task must be one of: all, t2v, i2v, r2v')
  }
  return args
}

function printHelp() {
  console.log(`Usage:
  node scripts/test-ali-aigc-video.mjs [options]

Dry-run is the default. The script only sends paid video requests when --run is present.

Options:
  --run                    Submit tasks to /v1/videos. Without this, only print the plan.
  --base-url URL           New API base URL. Default: NEWAPI_BASE_URL or http://localhost:3003
  --api-key KEY            New API key. Default: NEWAPI_API_KEY
  --image PATH             Local image path for base64/data-url dry-run or failure testing.
  --image-url URL          Public HTTPS or oss:// image URL for real I2V success testing.
  --use-base64-image       Convert --image to a data URL for I2V. Useful to expose unsupported base64 behavior.
  --duration N             Video duration in seconds. Default: 15
  --provider NAME          all, happyhorse, or kling. Default: all
  --task NAME              all, t2v, i2v, or r2v. Default: all
  --no-poll                Submit only, do not poll task completion.
  --t2v-only               Only include text-to-video cases.
  --i2v-only               Only include image-to-video cases.
`)
}

function mimeFromPath(filePath) {
  const ext = path.extname(filePath).toLowerCase()
  if (ext === '.jpg' || ext === '.jpeg') return 'image/jpeg'
  if (ext === '.png') return 'image/png'
  if (ext === '.webp') return 'image/webp'
  return 'application/octet-stream'
}

async function imagePathToDataUrl(filePath) {
  const bytes = await readFile(filePath)
  return `data:${mimeFromPath(filePath)};base64,${bytes.toString('base64')}`
}

function buildPrompts() {
  // Prompt follows the happyhorse-prompt-tuner guidance:
  // keep one readable timeline, preserve the source image composition for I2V,
  // and make physical actions visible instead of relying on vague style words.
  const t2vPrompt = `单镜头真实电影感科幻场景，雪山日出，云海翻涌，远处金色太阳低悬。一个黑色重型机甲站在积雪山脊上，肩部装甲厚重，双手握着超长电磁炮，炮口指向右侧远方。0-3秒：风雪掠过镜头，机甲保持低姿态，蓝色传感器微微亮起。3-6秒：机甲缓慢抬起头，肩甲和背部机械结构细微展开，脚下积雪被震落。6-9秒：机甲把巨型长炮平稳抬高并完成瞄准，炮身机械锁扣依次合拢。9-12秒：炮口开始聚集冷蓝色能量，雪粒和雾气被吸向炮口。12-15秒：镜头轻微低角度推近，机甲稳定蓄能但不发射，远处日出光线照亮金属边缘。动作连贯、重量感强、真实物理、无文字、无卡通风格。`

  const i2vPrompt = `保持首帧构图和主体身份：雪山日出、云海、黑色重型机甲、右侧超长电磁炮、低角度真实电影感画面都不改变。0-3秒：风雪从画面前景吹过，机甲蓝色传感器亮起，身体从静止中轻微启动。3-6秒：机甲肩部装甲和背部机械片缓慢展开，右腿压实雪地，雪粉被重量震起。6-9秒：机甲双手稳住超长电磁炮，炮身微微上抬并完成精准瞄准。9-12秒：炮口聚集冷蓝色能量，金属缝隙有细小电弧，周围雪粒被气流带动。12-15秒：镜头保持同一方向轻微推近，日出边缘光照亮装甲轮廓，机甲持续蓄能但不发射。不要改变机甲外形，不要换场景，不要出现人物或文字。`

  const r2vPrompt = `参考首帧中的机甲、雪山、日出和巨型长炮作为主体参考，不改变机甲身份、黑色厚重装甲、长炮朝右、雪山云海环境。0-3秒：画面从参考构图开始，风雪增强，机甲传感器亮起。3-6秒：机甲肩部装甲和背部机械结构轻微展开，身体重心下沉，雪地被压实。6-9秒：机甲保持长炮朝右，炮身细节机械锁扣依次合拢。9-12秒：炮口出现冷蓝色能量和细小电弧，周围雪粒被气流吸引。12-15秒：镜头保持真实低角度，轻微推近，日出边缘光照亮金属轮廓，机甲稳定蓄能但不发射。保持参考图主体一致，不要换机甲，不要新增人物，不要出现文字。`

  return { t2vPrompt, i2vPrompt, r2vPrompt }
}

function merge(...objects) {
  return Object.assign({}, ...objects)
}

function buildCases({ duration, includeT2V, includeI2V, includeR2V, provider }) {
  const { t2vPrompt, i2vPrompt, r2vPrompt } = buildPrompts()
  const baseMetadata = {
    duration,
    watermark: false,
    seed: 2026052017,
  }
  const happyHorseMetadata = {
    resolution: '720P',
    ratio: '16:9',
  }
  const klingMetadata = {
    mode: 'std',
    aspect_ratio: '16:9',
  }

  const cases = []
  if (includeT2V && (provider === 'all' || provider === 'happyhorse')) {
    cases.push({
      name: 'happyhorse-t2v',
      model: 'happyhorse-1.0-t2v',
      prompt: t2vPrompt,
      metadata: merge(baseMetadata, happyHorseMetadata),
      withImage: false,
    })
  }
  if (includeT2V && (provider === 'all' || provider === 'kling')) {
    cases.push({
      name: 'kling-v3-t2v',
      model: 'kling/kling-v3-video-generation',
      prompt: t2vPrompt,
      metadata: merge(baseMetadata, klingMetadata),
      withImage: false,
    })
    cases.push({
      name: 'kling-v3-omni-t2v',
      model: 'kling/kling-v3-omni-video-generation',
      prompt: t2vPrompt,
      metadata: merge(baseMetadata, klingMetadata),
      withImage: false,
    })
  }
  if (includeI2V && (provider === 'all' || provider === 'happyhorse')) {
    cases.push({
      name: 'happyhorse-i2v',
      model: 'happyhorse-1.0-i2v',
      prompt: i2vPrompt,
      metadata: merge(baseMetadata, happyHorseMetadata),
      withImage: true,
    })
  }
  if (includeR2V && (provider === 'all' || provider === 'happyhorse')) {
    cases.push({
      name: 'happyhorse-r2v',
      model: 'happyhorse-1.0-r2v',
      prompt: r2vPrompt,
      metadata: merge(baseMetadata, happyHorseMetadata),
      withImage: true,
    })
  }
  if (includeI2V && (provider === 'all' || provider === 'kling')) {
    cases.push({
      name: 'kling-v3-i2v',
      model: 'kling/kling-v3-video-generation',
      prompt: i2vPrompt,
      metadata: merge(baseMetadata, klingMetadata),
      withImage: true,
    })
    cases.push({
      name: 'kling-v3-omni-i2v',
      model: 'kling/kling-v3-omni-video-generation',
      prompt: i2vPrompt,
      metadata: merge(baseMetadata, klingMetadata),
      withImage: true,
    })
  }
  return cases
}

function redact(value) {
  if (!value) return ''
  if (value.length <= 12) return `${value.slice(0, 3)}...`
  return `${value.slice(0, 8)}...${value.slice(-4)}`
}

function summarizeBody(body) {
  return {
    model: body.model,
    duration: body.duration,
    seconds: body.seconds,
    hasImage: Boolean(body.image),
    imageKind: body.image
      ? body.image.startsWith('data:')
        ? 'data-url'
        : body.image.startsWith('oss://')
          ? 'oss-url'
          : body.image.startsWith('http')
            ? 'http-url'
            : 'raw'
      : '',
    metadata: body.metadata,
    prompt: `${body.prompt.slice(0, 90)}...`,
  }
}

async function submitVideo({ baseUrl, apiKey, body }) {
  const response = await fetch(`${baseUrl.replace(/\/$/, '')}/v1/videos`, {
    method: 'POST',
    headers: {
      authorization: `Bearer ${apiKey}`,
      'content-type': 'application/json; charset=utf-8',
    },
    body: JSON.stringify(body),
  })
  const text = await response.text()
  let payload
  try {
    payload = JSON.parse(text)
  } catch {
    payload = { raw: text }
  }
  if (!response.ok) {
    throw new Error(`submit failed (${response.status}): ${text}`)
  }
  return payload
}

async function pollVideo({ baseUrl, apiKey, taskId, pollIntervalMs, maxPolls }) {
  for (let i = 1; i <= maxPolls; i += 1) {
    const response = await fetch(`${baseUrl.replace(/\/$/, '')}/v1/videos/${taskId}`, {
      headers: { authorization: `Bearer ${apiKey}` },
    })
    const text = await response.text()
    let payload
    try {
      payload = JSON.parse(text)
    } catch {
      payload = { raw: text }
    }
    if (!response.ok) {
      throw new Error(`poll failed (${response.status}): ${text}`)
    }
    const url = payload?.metadata?.url || ''
    console.log(
      `[${i}/${maxPolls}] ${taskId} status=${payload.status} progress=${payload.progress ?? ''} url=${url}`
    )
    if (['completed', 'succeeded', 'success', 'failed', 'cancelled'].includes(payload.status)) {
      return payload
    }
    await new Promise((resolve) => setTimeout(resolve, pollIntervalMs))
  }
  throw new Error(`task did not finish after ${maxPolls} polls: ${taskId}`)
}

async function main() {
  const args = parseArgs(process.argv.slice(2))
  const includeT2V = args.task === 'all' || args.task === 't2v' ? !args.i2vOnly : false
  const includeI2V = args.task === 'all' || args.task === 'i2v' ? !args.t2vOnly : false
  const includeR2V = args.task === 'all' || args.task === 'r2v' ? !args.t2vOnly : false
  const cases = buildCases({
    duration: args.duration,
    includeT2V,
    includeI2V,
    includeR2V,
    provider: args.provider,
  })

  let imagePayload = ''
  if (args.imageUrl) {
    imagePayload = args.imageUrl
  } else if (args.useBase64Image) {
    imagePayload = await imagePathToDataUrl(args.image)
  }

  const bodies = cases.map((item) => {
    const body = {
      model: item.model,
      prompt: item.prompt,
      duration: args.duration,
      seconds: args.duration,
      metadata: item.metadata,
    }
    if (item.withImage) {
      body.image = imagePayload
      body.images = imagePayload ? [imagePayload] : []
    }
    return { ...item, body }
  })

  console.log(`Base URL: ${args.baseUrl}`)
  console.log(`API key: ${redact(args.apiKey) || '(missing)'}`)
  console.log(`Duration: ${args.duration}s`)
  console.log(`Mode: ${args.run ? 'RUN, paid requests may be sent' : 'DRY RUN, no requests will be sent'}`)
  if (includeI2V && !imagePayload) {
    console.warn(
      'I2V cases need --image-url with public HTTPS/oss:// URL. Use --use-base64-image only to test current base64 failure behavior.'
    )
  }
  console.log('')

  for (const item of bodies) {
    console.log(`Case: ${item.name}`)
    console.log(JSON.stringify(summarizeBody(item.body), null, 2))
    console.log('')
  }

  if (!args.run) {
    console.log('Dry run complete. Add --run to submit tasks.')
    return
  }
  if (!args.apiKey) {
    throw new Error('Missing API key. Pass --api-key or set NEWAPI_API_KEY.')
  }

  const results = []
  for (const item of bodies) {
    if (item.withImage && !imagePayload) {
      results.push({
        name: item.name,
        model: item.model,
        status: 'skipped',
        error: 'missing --image-url or --use-base64-image',
      })
      continue
    }
    console.log(`Submitting ${item.name} (${item.model})`)
    try {
      const submitted = await submitVideo({ baseUrl: args.baseUrl, apiKey: args.apiKey, body: item.body })
      const taskId = submitted.task_id || submitted.id
      let final = submitted
      if (args.poll && taskId) {
        final = await pollVideo({
          baseUrl: args.baseUrl,
          apiKey: args.apiKey,
          taskId,
          pollIntervalMs: args.pollIntervalMs,
          maxPolls: args.maxPolls,
        })
      }
      results.push({
        name: item.name,
        model: item.model,
        task_id: taskId,
        status: final.status,
        url: final?.metadata?.url || '',
        error: final?.error?.message || '',
      })
    } catch (error) {
      results.push({
        name: item.name,
        model: item.model,
        status: 'error',
        error: error.message,
      })
    }
    console.log('')
  }

  console.table(results)
}

main().catch((error) => {
  console.error(error.message)
  process.exitCode = 1
})

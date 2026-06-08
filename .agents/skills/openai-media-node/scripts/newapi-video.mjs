import axios from 'axios';
import fs from 'node:fs';
import path from 'node:path';
import {
  getApiKey,
  handleError,
  optionalInteger,
  parseArgs,
  parseJsonArg,
  pick,
  requireArg,
} from './media-common.mjs';

const terminalStatuses = new Set(['completed', 'failed']);

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function merge(target, source) {
  if (!source) return target;
  for (const [key, value] of Object.entries(source)) {
    target[key] = value;
  }
  return target;
}

function dataUrlFromFile(filePath, mediaKind = 'image') {
  const resolved = path.resolve(String(filePath));
  const ext = path.extname(resolved).toLowerCase();
  const mimeByExt = {
    '.png': 'image/png',
    '.webp': 'image/webp',
    '.gif': 'image/gif',
    '.bmp': 'image/bmp',
    '.tif': 'image/tiff',
    '.tiff': 'image/tiff',
    '.heic': 'image/heic',
    '.heif': 'image/heif',
    '.mp4': 'video/mp4',
    '.mov': 'video/quicktime',
    '.mp3': 'audio/mpeg',
    '.wav': 'audio/wav',
  };
  const fallback =
    mediaKind === 'video'
      ? 'video/mp4'
      : mediaKind === 'audio'
        ? 'audio/wav'
        : 'image/jpeg';
  const mime = mimeByExt[ext] || fallback;
  return `data:${mime};base64,${fs.readFileSync(resolved).toString('base64')}`;
}

function splitList(value) {
  if (value === undefined || value === true || value === '') return [];
  return String(value)
    .split(';')
    .map((item) => item.trim())
    .filter(Boolean);
}

function mediaFromArgs(args, mode) {
  const explicit = parseJsonArg(args, 'media-json');
  if (explicit) return explicit;
  const media = [];
  if (args['video-url']) {
    media.push({
      type: mode === 'edit' ? 'video' : 'reference_video',
      url: String(args['video-url']),
    });
  }
  if (args['video-file']) {
    media.push({
      type: mode === 'edit' ? 'video' : 'reference_video',
      url: dataUrlFromFile(args['video-file'], 'video'),
    });
  }
  if (args['reference-video-url']) {
    media.push({
      type: 'reference_video',
      url: String(args['reference-video-url']),
    });
  }
  if (args['reference-video-file']) {
    media.push({
      type: 'reference_video',
      url: dataUrlFromFile(args['reference-video-file'], 'video'),
    });
  }
  if (args['image-url']) {
    media.push({
      type: mode === 'i2v' ? 'first_frame' : 'reference_image',
      url: String(args['image-url']),
    });
  }
  if (args['image-file']) {
    media.push({
      type: mode === 'i2v' ? 'first_frame' : 'reference_image',
      url: dataUrlFromFile(args['image-file']),
    });
  }
  if (args['first-frame-url']) {
    media.push({
      type: 'first_frame',
      url: String(args['first-frame-url']),
    });
  }
  if (args['first-frame-file']) {
    media.push({
      type: 'first_frame',
      url: dataUrlFromFile(args['first-frame-file']),
    });
  }
  if (args['last-frame-url']) {
    media.push({
      type: 'last_frame',
      url: String(args['last-frame-url']),
    });
  }
  if (args['last-frame-file']) {
    media.push({
      type: 'last_frame',
      url: dataUrlFromFile(args['last-frame-file']),
    });
  }
  if (args['reference-image-url']) {
    media.push({
      type: 'reference_image',
      url: String(args['reference-image-url']),
    });
  }
  if (args['reference-image-file']) {
    media.push({
      type: 'reference_image',
      url: dataUrlFromFile(args['reference-image-file']),
    });
  }
  for (const file of splitList(args['reference-image-files'])) {
    media.push({
      type: 'reference_image',
      url: dataUrlFromFile(file),
    });
  }
  if (args['audio-url']) {
    media.push({
      type: 'reference_audio',
      url: String(args['audio-url']),
    });
  }
  if (args['audio-file']) {
    media.push({
      type: 'reference_audio',
      url: dataUrlFromFile(args['audio-file'], 'audio'),
    });
  }
  if (args['reference-audio-url']) {
    media.push({
      type: 'reference_audio',
      url: String(args['reference-audio-url']),
    });
  }
  if (args['reference-audio-file']) {
    media.push({
      type: 'reference_audio',
      url: dataUrlFromFile(args['reference-audio-file'], 'audio'),
    });
  }
  return media;
}

function normalizeDoubaoResolution(value) {
  if (value === undefined) return undefined;
  return String(value).trim().toLowerCase();
}

function doubaoImageRole(item, mode) {
  if (item.role) return item.role;
  if (item.type === 'first_frame') return 'first_frame';
  if (item.type === 'last_frame') return 'last_frame';
  if (mode === 'i2v') return 'first_frame';
  return 'reference_image';
}

function optionalBoolean(args, key) {
  if (args[key] === undefined || args[key] === '') return undefined;
  if (args[key] === true) return true;
  return String(args[key]).toLowerCase() === 'true';
}

function doubaoMetadata(args, mode, prompt) {
  const content = parseJsonArg(args, 'content-json') || [];
  for (const item of mediaFromArgs(args, mode)) {
    if (item.type === 'video' || item.type === 'reference_video') {
      content.push({ type: 'video_url', video_url: { url: item.url }, role: item.role || 'reference_video' });
    } else if (item.type === 'audio' || item.type === 'reference_audio') {
      content.push({ type: 'audio_url', audio_url: { url: item.url }, role: item.role || 'reference_audio' });
    } else {
      content.push({ type: 'image_url', image_url: { url: item.url }, role: doubaoImageRole(item, mode) });
    }
  }
  if (args['draft-task-id']) {
    content.push({ type: 'draft_task', draft_task: { id: String(args['draft-task-id']) } });
  }
  content.push({ type: 'text', text: prompt });
  const metadata = {
    content,
    callback_url: pick(args, 'callback-url', undefined),
    return_last_frame: optionalBoolean(args, 'return-last-frame'),
    service_tier: pick(args, 'service-tier', undefined),
    execution_expires_after: optionalInteger(args, 'execution-expires-after'),
    resolution: normalizeDoubaoResolution(pick(args, 'resolution', undefined)),
    ratio: pick(args, 'ratio', undefined),
    frames: optionalInteger(args, 'frames'),
    seed: optionalInteger(args, 'seed'),
    camera_fixed: optionalBoolean(args, 'camera-fixed'),
    watermark: optionalBoolean(args, 'watermark'),
    draft: optionalBoolean(args, 'draft'),
    safety_identifier: pick(args, 'safety-identifier', undefined),
    priority: optionalInteger(args, 'priority'),
  };
  if (args['generate-audio'] !== undefined) {
    metadata.generate_audio = optionalBoolean(args, 'generate-audio');
  }
  const toolsJson = parseJsonArg(args, 'tools-json');
  if (toolsJson) {
    metadata.tools = toolsJson;
  } else if (optionalBoolean(args, 'web-search') !== undefined) {
    metadata.tools = optionalBoolean(args, 'web-search') ? [{ type: 'web_search' }] : [];
  }
  return metadata;
}

function happyHorseMetadata(args, mode, prompt) {
  const input = merge({ prompt }, parseJsonArg(args, 'input-json'));
  const parameters = merge({}, parseJsonArg(args, 'parameters-json'));
  if (!input.media) {
    const media = mediaFromArgs(args, mode).map((item, index) => ({
      type:
        mode === 'i2v'
          ? 'first_frame'
          : mode === 'edit' && index === 0
            ? 'video'
            : 'reference_image',
      url: item.url,
    }));
    if (media.length > 0) input.media = media;
  }
  parameters.resolution = pick(args, 'resolution', parameters.resolution);
  parameters.size = pick(args, 'size', parameters.size);
  parameters.watermark =
    args.watermark === undefined ? (parameters.watermark ?? false) : args.watermark === 'true';
  return { input, parameters };
}

function klingMetadata(args, mode) {
  const metadata = {
    image_tail: pick(args, 'image-tail-url', undefined),
    mode: pick(args, 'generation-mode', undefined),
    aspect_ratio: pick(args, 'aspect-ratio', undefined),
    negative_prompt: pick(args, 'negative-prompt', undefined),
  };
  if (mode === 'r2v' || mode === 'edit') {
    const media = mediaFromArgs(args, mode);
    if (media.length > 0) {
      metadata.input = { media };
    }
  }
  return metadata;
}

function providerMetadata(args, provider, mode, prompt) {
  if (provider === 'doubao') return doubaoMetadata(args, mode, prompt);
  if (provider === 'happyhorse') return happyHorseMetadata(args, mode, prompt);
  if (provider === 'kling') return klingMetadata(args, mode);
  return {};
}

function buildPayload(args) {
  const provider = pick(args, 'provider', '').toLowerCase();
  const mode = pick(args, 'mode', 't2v').toLowerCase();
  const prompt = requireArg(args, 'prompt');
  const duration = optionalInteger(args, 'duration') ?? optionalInteger(args, 'seconds');
  const metadata = merge(
    providerMetadata(args, provider, mode, prompt),
    parseJsonArg(args, 'metadata-json')
  );

  const payload = {
    model: requireArg(args, 'model'),
    prompt,
    image: pick(args, 'image-url', undefined),
    duration,
    resolution: pick(args, 'resolution', undefined),
    width: optionalInteger(args, 'width'),
    height: optionalInteger(args, 'height'),
    fps: optionalInteger(args, 'fps'),
    seed: optionalInteger(args, 'seed'),
    n: optionalInteger(args, 'n'),
    response_format: pick(args, 'response-format', undefined),
    user: pick(args, 'user', undefined),
    metadata,
  };

  if (mode === 't2v' && provider !== 'doubao') {
    delete payload.image;
  }

  merge(payload, parseJsonArg(args, 'extra-json'));
  Object.keys(payload).forEach((key) => {
    if (
      payload[key] === undefined ||
      (key === 'metadata' && Object.keys(payload[key]).length === 0)
    ) {
      delete payload[key];
    }
  });
  return payload;
}

function createAxios(args) {
  const baseURL = pick(args, 'base-url', process.env.NEWAPI_BASE_URL || 'http://localhost:3000');
  const apiKey = getApiKey('video');
  return axios.create({
    baseURL: baseURL.replace(/\/$/, ''),
    timeout: optionalInteger(args, 'request-timeout') ?? 10 * 60 * 1000,
    headers: {
      Authorization: `Bearer ${apiKey}`,
      'Content-Type': 'application/json',
      Accept: 'application/json',
    },
  });
}

async function main() {
  const args = parseArgs();
  const client = createAxios(args);
  const pollInterval = optionalInteger(args, 'poll-interval') ?? 10000;
  const timeoutMs = optionalInteger(args, 'timeout') ?? 30 * 60 * 1000;
  const payload = buildPayload(args);

  const createResp = await client.post('/v1/video/generations', payload);
  const taskID = createResp.data?.task_id || createResp.data?.id;
  if (!taskID) {
    throw new Error(`Create response did not include task_id: ${JSON.stringify(createResp.data)}`);
  }
  console.error(`started new-api task ${taskID} status=${createResp.data?.status || 'unknown'}`);

  if (args['no-poll']) {
    console.log(JSON.stringify(createResp.data, null, 2));
    return;
  }

  const deadline = Date.now() + timeoutMs;
  let latest;
  while (Date.now() <= deadline) {
    await sleep(pollInterval);
    const getResp = await client.get(`/v1/video/generations/${encodeURIComponent(taskID)}`);
    latest = getResp.data;
    console.error(`status=${latest?.status || 'unknown'}`);
    if (terminalStatuses.has(latest?.status)) break;
  }

  if (!latest || !terminalStatuses.has(latest.status)) {
    throw new Error(`Timed out waiting for ${taskID}`);
  }
  if (latest.status !== 'completed') {
    throw new Error(`Task ${taskID} failed: ${JSON.stringify(latest)}`);
  }
  console.log(JSON.stringify(latest, null, 2));
}

main().catch(handleError);

import axios from 'axios';
import fs from 'node:fs';
import path from 'node:path';
import {
  ensureDirFor,
  getBaseURL,
  getVideoApiKey,
  handleError,
  loadEnv,
  optionalInteger,
  parseArgs,
  parseJsonArg,
  pick,
  requireArg,
} from './media-common.mjs';

const terminalStatuses = new Set([
  'completed',
  'succeeded',
  'success',
  'failed',
  'error',
  'canceled',
  'cancelled',
  'expired',
]);
const successStatuses = new Set(['completed', 'succeeded', 'success']);
const failureStatuses = new Set(['failed', 'error', 'canceled', 'cancelled', 'expired']);

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
  return 'reference_image';
}

function optionalBoolean(args, key) {
  if (args[key] === undefined || args[key] === '') return undefined;
  if (args[key] === true) return true;
  return String(args[key]).toLowerCase() === 'true';
}

function promptFromArgs(args) {
  if (args['prompt-file']) {
    return fs.readFileSync(path.resolve(String(args['prompt-file'])), 'utf8').trim();
  }
  return requireArg(args, 'prompt');
}

function validateProviderArgs(provider, args) {
  if (provider !== 'happyhorse') return;
  const duration = optionalInteger(args, 'duration') ?? optionalInteger(args, 'seconds');
  if (duration !== undefined && duration < 3) {
    throw new Error('HappyHorse duration must be at least 3 seconds. Use --duration 3 or higher.');
  }
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
  parameters.duration =
    optionalInteger(args, 'duration') ?? optionalInteger(args, 'seconds') ?? parameters.duration;
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
  validateProviderArgs(provider, args);
  const prompt = promptFromArgs(args);
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
  loadEnv();
  const baseURL = pick(args, 'base-url', getBaseURL('video') || 'http://localhost:3000');
  const apiKey = getVideoApiKey({
    model: args.model,
    provider: args.provider,
  });
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

function saveJson(filePath, value) {
  if (!filePath) return;
  ensureDirFor(filePath);
  fs.writeFileSync(path.resolve(String(filePath)), `${JSON.stringify(value, null, 2)}\n`);
}

function videoUrlFromResponse(json) {
  return (
    json?.url ||
    json?.metadata?.url ||
    json?.result_url ||
    json?.data?.url ||
    json?.data?.metadata?.url ||
    json?.data?.result_url ||
    json?.data?.output?.video_url ||
    json?.data?.data?.output?.video_url ||
    json?.output?.video_url ||
    json?.response?.url ||
    json?.response?.metadata?.url
  );
}

function taskStatusFromResponse(json) {
  const raw =
    json?.status ||
    json?.data?.status ||
    json?.data?.task_status ||
    json?.data?.data?.output?.task_status ||
    json?.output?.task_status ||
    json?.response?.status ||
    json?.response?.data?.status ||
    json?.response?.data?.output?.task_status;
  if (!raw) return undefined;
  const normalized = String(raw).trim().toLowerCase();
  if (normalized === 'success' || normalized === 'succeeded') return 'succeeded';
  if (normalized === 'failed' || normalized === 'fail') return 'failed';
  if (normalized === 'cancelled') return 'canceled';
  return normalized;
}

function taskProgressFromResponse(json) {
  return (
    json?.progress ??
    json?.data?.progress ??
    json?.data?.data?.progress ??
    json?.response?.progress ??
    json?.response?.data?.progress
  );
}

async function fetchTask(client, taskID) {
  const pathName = `/v1/video/generations/${encodeURIComponent(taskID)}`;
  const response = await client.get(pathName);
  return { path: pathName, data: response.data };
}

async function downloadFile(url, outPath) {
  if (!url || !outPath) return;
  ensureDirFor(outPath);
  const response = await axios.get(url, {
    responseType: 'arraybuffer',
    timeout: 10 * 60 * 1000,
    validateStatus: () => true,
  });
  if (response.status < 200 || response.status >= 300) {
    throw new Error(`Download HTTP ${response.status}: ${JSON.stringify(response.data)}`);
  }
  fs.writeFileSync(path.resolve(String(outPath)), Buffer.from(response.data));
}

async function main() {
  const args = parseArgs();
  const pollInterval = optionalInteger(args, 'poll-interval') ?? 10000;
  const timeoutMs = optionalInteger(args, 'timeout') ?? 30 * 60 * 1000;

  if (args['task-id']) {
    const client = createAxios(args);
    const taskID = requireArg(args, 'task-id');
    const fetched = await fetchTask(client, taskID);
    const latest = fetched.data;
    const status = taskStatusFromResponse(latest);
    console.error(`status=${status || 'unknown'} path=${fetched.path}`);
    const url = videoUrlFromResponse(latest);
    const result = { ...latest, url: latest.url || url };
    saveJson(pick(args, 'response-out', undefined), result);
    if (successStatuses.has(status)) {
      await downloadFile(url, pick(args, 'out', undefined));
    }
    console.log(JSON.stringify(result, null, 2));
    return;
  }

  const payload = buildPayload(args);
  saveJson(pick(args, 'request-out', undefined), payload);

  if (args['dry-run']) {
    console.log(JSON.stringify(payload, null, 2));
    return;
  }

  const client = createAxios(args);
  let createResp;
  try {
    createResp = await client.post('/v1/video/generations', payload);
  } catch (error) {
    const errorPayload = {
      error: {
        message: error?.message || String(error),
        status: error?.status,
        code: error?.code,
        response_status: error?.response?.status,
        response_data: error?.response?.data,
      },
    };
    saveJson(pick(args, 'create-response-out', undefined), errorPayload);
    saveJson(pick(args, 'response-out', undefined), errorPayload);
    throw error;
  }
  saveJson(pick(args, 'create-response-out', undefined), createResp.data);
  const taskID = createResp.data?.task_id || createResp.data?.id;
  const createStatus = taskStatusFromResponse(createResp.data);
  if (createStatus && failureStatuses.has(createStatus)) {
    saveJson(pick(args, 'response-out', undefined), createResp.data);
    throw new Error(`Create request failed: ${JSON.stringify(createResp.data)}`);
  }
  if (!taskID) {
    saveJson(pick(args, 'response-out', undefined), createResp.data);
    throw new Error(`Create response did not include task_id: ${JSON.stringify(createResp.data)}`);
  }
  console.error(`started new-api task ${taskID} status=${createStatus || 'unknown'}`);

  if (args['no-poll']) {
    saveJson(pick(args, 'response-out', undefined), createResp.data);
    console.log(JSON.stringify(createResp.data, null, 2));
    return;
  }

  const deadline = Date.now() + timeoutMs;
  let latest;
  while (Date.now() <= deadline) {
    await sleep(pollInterval);
    const fetched = await fetchTask(client, taskID);
    latest = fetched.data;
    const status = taskStatusFromResponse(latest);
    const progress = taskProgressFromResponse(latest);
    console.error(
      `status=${status || 'unknown'}${progress === undefined ? '' : ` progress=${progress}`} path=${fetched.path}`
    );
    if (status && terminalStatuses.has(status)) break;
  }

  const finalStatus = taskStatusFromResponse(latest);
  if (!latest || !terminalStatuses.has(finalStatus)) {
    throw new Error(`Timed out waiting for ${taskID}`);
  }
  if (!successStatuses.has(finalStatus)) {
    saveJson(pick(args, 'response-out', undefined), latest);
    throw new Error(`Task ${taskID} failed: ${JSON.stringify(latest)}`);
  }
  const url = videoUrlFromResponse(latest);
  const result = { ...latest, url: latest.url || url };
  saveJson(pick(args, 'response-out', undefined), result);
  await downloadFile(url, pick(args, 'out', undefined));
  console.log(JSON.stringify(result, null, 2));
}

main()
  .then(() => {
    process.exit(0);
  })
  .catch((error) => {
    handleError(error);
    process.exit(1);
  });

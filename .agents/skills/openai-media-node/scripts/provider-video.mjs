import crypto from 'node:crypto';
import axios from 'axios';
import {
  handleError,
  optionalInteger,
  parseArgs,
  parseJsonArg,
  pick,
  requireArg,
} from './media-common.mjs';

const terminalStatuses = new Set([
  'completed',
  'succeeded',
  'SUCCEEDED',
  'failed',
  'FAILED',
  'CANCELED',
  'cancelled',
  'expired',
]);

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

function commonMedia(args, mode) {
  const explicit = parseJsonArg(args, 'media-json');
  if (explicit) return explicit;
  const media = [];
  if (args['video-url']) {
    media.push({
      type: mode === 'edit' ? 'video' : 'reference_video',
      url: String(args['video-url']),
    });
  }
  const imageUrls = []
    .concat(args['image-url'] ? [String(args['image-url'])] : [])
    .concat(args['reference-image-url'] ? [String(args['reference-image-url'])] : []);
  for (const url of imageUrls) {
    media.push({
      type: mode === 'i2v' ? 'first_frame' : 'reference_image',
      url,
    });
  }
  return media;
}

function doubaoPayload(args) {
  const mode = pick(args, 'mode', 't2v');
  const content = parseJsonArg(args, 'content-json') || [];
  for (const item of commonMedia(args, mode)) {
    if (item.type === 'video' || item.type === 'reference_video') {
      content.push({ type: 'video_url', video_url: { url: item.url } });
    } else {
      content.push({ type: 'image_url', image_url: { url: item.url } });
    }
  }
  content.push({ type: 'text', text: requireArg(args, 'prompt') });
  const payload = {
    model: requireArg(args, 'model'),
    content,
    duration: optionalInteger(args, 'duration') ?? optionalInteger(args, 'seconds'),
    resolution: pick(args, 'resolution', undefined),
    ratio: pick(args, 'ratio', undefined),
  };
  merge(payload, parseJsonArg(args, 'extra-json'));
  Object.keys(payload).forEach((key) => payload[key] === undefined && delete payload[key]);
  return payload;
}

function happyHorsePayload(args) {
  const mode = pick(args, 'mode', 't2v');
  const input = merge({ prompt: requireArg(args, 'prompt') }, parseJsonArg(args, 'input-json'));
  const parameters = merge({}, parseJsonArg(args, 'parameters-json'));
  if (!input.media) {
    input.media = commonMedia(args, mode).map((item, index) => ({
      type:
        mode === 'i2v'
          ? 'first_frame'
          : mode === 'edit' && index === 0
            ? 'video'
            : 'reference_image',
      url: item.url,
    }));
    if (input.media.length === 0) delete input.media;
  }
  parameters.duration =
    optionalInteger(args, 'duration') ?? optionalInteger(args, 'seconds') ?? parameters.duration;
  parameters.resolution = pick(args, 'resolution', parameters.resolution);
  parameters.size = pick(args, 'size', parameters.size);
  const payload = {
    model: requireArg(args, 'model'),
    input,
    parameters,
  };
  merge(payload, parseJsonArg(args, 'extra-json'));
  return payload;
}

function klingJwt(apiKey) {
  if (!apiKey.includes('|')) return apiKey;
  const [accessKey, secretKey] = apiKey.split('|').map((v) => v.trim());
  const now = Math.floor(Date.now() / 1000);
  const header = Buffer.from(JSON.stringify({ alg: 'HS256', typ: 'JWT' })).toString('base64url');
  const payload = Buffer.from(
    JSON.stringify({ iss: accessKey, exp: now + 1800, nbf: now - 5 })
  ).toString('base64url');
  const signature = crypto
    .createHmac('sha256', secretKey)
    .update(`${header}.${payload}`)
    .digest('base64url');
  return `${header}.${payload}.${signature}`;
}

function klingPayload(args) {
  const mode = pick(args, 'mode', 't2v');
  const payload = {
    model_name: requireArg(args, 'model'),
    model: pick(args, 'model', undefined),
    prompt: requireArg(args, 'prompt'),
    image: pick(args, 'image-url', undefined),
    image_tail: pick(args, 'image-tail-url', undefined),
    duration: String(optionalInteger(args, 'duration') ?? optionalInteger(args, 'seconds') ?? 5),
    mode: pick(args, 'generation-mode', 'std'),
    aspect_ratio: pick(args, 'aspect-ratio', undefined),
  };
  if (mode === 't2v') {
    delete payload.image;
    delete payload.image_tail;
  }
  merge(payload, parseJsonArg(args, 'extra-json'));
  Object.keys(payload).forEach((key) => payload[key] === undefined && delete payload[key]);
  return payload;
}

function providerConfig(args) {
  const provider = requireArg(args, 'provider').toLowerCase();
  if (provider === 'doubao') {
    const baseURL = pick(args, 'base-url', process.env.DOUBAO_VIDEO_BASE_URL || 'https://ark.cn-beijing.volces.com');
    const apiKey = pick(args, 'api-key', undefined);
    if (!apiKey) throw new Error('Missing --api-key for direct Doubao provider call');
    return {
      provider,
      createURL: `${baseURL.replace(/\/$/, '')}/api/v3/contents/generations/tasks`,
      fetchURL: (id) => `${baseURL.replace(/\/$/, '')}/api/v3/contents/generations/tasks/${id}`,
      headers: { Authorization: `Bearer ${apiKey}` },
      payload: doubaoPayload(args),
      idFromCreate: (json) => json.id,
      statusFromFetch: (json) => json.status,
      urlFromFetch: (json) => json.content?.video_url,
    };
  }
  if (provider === 'happyhorse') {
    const baseURL = pick(args, 'base-url', process.env.HAPPYHORSE_BASE_URL || 'https://dashscope.aliyuncs.com');
    const apiKey = pick(args, 'api-key', undefined);
    if (!apiKey) throw new Error('Missing --api-key for direct HappyHorse provider call');
    return {
      provider,
      createURL: `${baseURL.replace(/\/$/, '')}/api/v1/services/aigc/video-generation/video-synthesis`,
      fetchURL: (id) => `${baseURL.replace(/\/$/, '')}/api/v1/tasks/${id}`,
      headers: {
        Authorization: `Bearer ${apiKey}`,
        'X-DashScope-Async': 'enable',
        'X-DashScope-OssResourceResolve': 'enable',
      },
      payload: happyHorsePayload(args),
      idFromCreate: (json) => json.output?.task_id,
      statusFromFetch: (json) => json.output?.task_status,
      urlFromFetch: (json) => json.output?.video_url,
    };
  }
  if (provider === 'kling') {
    const mode = pick(args, 'mode', 't2v');
    const action = mode === 't2v' ? 'text2video' : 'image2video';
    const baseURL = pick(args, 'base-url', process.env.KLING_BASE_URL || 'https://api.klingai.com');
    const key = pick(args, 'api-key', undefined);
    if (!key) throw new Error('Missing --api-key for direct Kling provider call');
    return {
      provider,
      createURL: `${baseURL.replace(/\/$/, '')}/v1/videos/${action}`,
      fetchURL: (id) => `${baseURL.replace(/\/$/, '')}/v1/videos/${action}/${id}`,
      headers: { Authorization: `Bearer ${klingJwt(key)}`, 'User-Agent': 'kling-sdk/1.0' },
      payload: klingPayload(args),
      idFromCreate: (json) => json.data?.task_id || json.task_id,
      statusFromFetch: (json) => json.data?.task_status || json.task_status,
      urlFromFetch: (json) => json.data?.task_result?.videos?.[0]?.url,
    };
  }
  throw new Error(`Unsupported --provider ${provider}`);
}

async function requestJSON(url, options) {
  try {
    const response = await axios.request({
      url,
      method: options.method,
      headers: options.headers,
      data: options.body ? JSON.parse(options.body) : undefined,
      timeout: options.timeout,
      validateStatus: () => true,
    });
    if (response.status < 200 || response.status >= 300) {
      throw new Error(`HTTP ${response.status}: ${JSON.stringify(response.data)}`);
    }
    return response.data || {};
  } catch (error) {
    if (error.response) {
      throw new Error(
        `HTTP ${error.response.status}: ${JSON.stringify(error.response.data)}`
      );
    }
    throw error;
  }
}

async function main() {
  const args = parseArgs();
  const config = providerConfig(args);
  const pollInterval = optionalInteger(args, 'poll-interval') ?? 10000;
  const timeoutMs = optionalInteger(args, 'timeout') ?? 30 * 60 * 1000;
  const requestTimeout = optionalInteger(args, 'request-timeout') ?? 10 * 60 * 1000;
  const create = await requestJSON(config.createURL, {
    method: 'POST',
    headers: {
      Accept: 'application/json',
      'Content-Type': 'application/json',
      ...config.headers,
    },
    body: JSON.stringify(config.payload),
    timeout: requestTimeout,
  });
  const taskID = config.idFromCreate(create);
  if (!taskID) throw new Error(`Create response did not include task id: ${JSON.stringify(create)}`);
  console.error(`started ${config.provider} task ${taskID}`);

  if (args['no-poll']) {
    console.log(JSON.stringify(create, null, 2));
    return;
  }

  const deadline = Date.now() + timeoutMs;
  let latest;
  while (Date.now() <= deadline) {
    await sleep(pollInterval);
    latest = await requestJSON(config.fetchURL(taskID), {
      method: 'GET',
      headers: { Accept: 'application/json', ...config.headers },
      timeout: requestTimeout,
    });
    const status = config.statusFromFetch(latest);
    console.error(`status=${status || 'unknown'}`);
    if (terminalStatuses.has(status)) break;
  }

  const status = config.statusFromFetch(latest);
  if (!terminalStatuses.has(status)) throw new Error(`Timed out waiting for ${taskID}`);
  if (!['completed', 'succeeded', 'SUCCEEDED'].includes(status)) {
    throw new Error(`Task ${taskID} failed with status=${status}: ${JSON.stringify(latest)}`);
  }
  const url = config.urlFromFetch(latest);
  console.log(JSON.stringify({ task_id: taskID, status, url, response: latest }, null, 2));
}

main().catch(handleError);

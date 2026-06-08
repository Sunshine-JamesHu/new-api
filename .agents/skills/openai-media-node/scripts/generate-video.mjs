import fs from 'node:fs';
import {
  createClient,
  ensureDirFor,
  handleError,
  logRequestId,
  optionalInteger,
  parseArgs,
  parseJsonArg,
  pick,
  requireArg,
} from './media-common.mjs';

const terminalStatuses = new Set(['completed', 'failed', 'cancelled', 'expired']);

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function buildInputReference(args) {
  if (args['input-reference']) {
    return fs.createReadStream(String(args['input-reference']));
  }
  if (args['input-reference-url']) {
    return { image_url: String(args['input-reference-url']) };
  }
  if (args['input-reference-file-id']) {
    return { file_id: String(args['input-reference-file-id']) };
  }
  return undefined;
}

async function main() {
  const args = parseArgs();
  const prompt = requireArg(args, 'prompt');
  const model = pick(args, 'model', process.env.OPENAI_VIDEO_MODEL || 'sora-2');
  const out = pick(args, 'out', `outputs/video-${Date.now()}.mp4`);
  const pollInterval = optionalInteger(args, 'poll-interval') ?? 10000;
  const timeoutMs = optionalInteger(args, 'timeout') ?? 30 * 60 * 1000;

  const request = {
    model,
    prompt,
    size: pick(args, 'size', undefined),
    seconds: pick(args, 'seconds', undefined),
    input_reference: buildInputReference(args),
    characters: parseJsonArg(args, 'characters-json'),
  };

  Object.keys(request).forEach((key) => {
    if (request[key] === undefined) delete request[key];
  });

  const openai = createClient('video');
  let video = await openai.videos.create(request);
  logRequestId(video);
  console.error(`started ${video.id} status=${video.status}`);

  const deadline = Date.now() + timeoutMs;
  while (!terminalStatuses.has(video.status)) {
    if (Date.now() > deadline) {
      throw new Error(`Timed out waiting for video ${video.id}`);
    }
    await sleep(pollInterval);
    video = await openai.videos.retrieve(video.id);
    const progress =
      video.progress === undefined || video.progress === null
        ? ''
        : ` progress=${video.progress}%`;
    console.error(`status=${video.status}${progress}`);
  }

  if (video.status !== 'completed') {
    throw new Error(
      `Video ${video.id} ended with status=${video.status}: ${JSON.stringify(
        video.error || {}
      )}`
    );
  }

  const content = await openai.videos.downloadContent(video.id);
  const buffer = Buffer.from(await content.arrayBuffer());
  ensureDirFor(out);
  fs.writeFileSync(out, buffer);
  console.log(out);
}

main().catch(handleError);

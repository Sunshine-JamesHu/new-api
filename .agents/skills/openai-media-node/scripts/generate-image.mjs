import fs from 'node:fs';
import {
  createClient,
  ensureDirFor,
  handleError,
  logRequestId,
  optionalInteger,
  parseArgs,
  pick,
  requireArg,
} from './media-common.mjs';

async function main() {
  const args = parseArgs();
  const prompt = requireArg(args, 'prompt');
  const model = pick(args, 'model', process.env.OPENAI_IMAGE_MODEL || 'gpt-image-2');
  const format = pick(args, 'format', 'png');
  const out = pick(args, 'out', `outputs/image-${Date.now()}.${format}`);
  const n = optionalInteger(args, 'n') ?? 1;
  const partialImages = optionalInteger(args, 'partial-images');

  const request = {
    model,
    prompt,
    n,
    size: pick(args, 'size', undefined),
    quality: pick(args, 'quality', undefined),
    output_format: pick(args, 'format', undefined),
    output_compression: optionalInteger(args, 'compression'),
    background: pick(args, 'background', undefined),
    moderation: pick(args, 'moderation', undefined),
  };

  Object.keys(request).forEach((key) => {
    if (request[key] === undefined) delete request[key];
  });

  const openai = createClient('image');

  if (args.stream) {
    const streamRequest = { ...request, stream: true };
    if (partialImages !== undefined) {
      streamRequest.partial_images = partialImages;
    }
    const stream = await openai.images.generate(streamRequest);
    let finalIndex = 0;
    for await (const event of stream) {
      if (event.type !== 'image_generation.partial_image') continue;
      const index = event.partial_image_index ?? finalIndex;
      const partialOut = out.replace(/(\.[^.]+)?$/, `-partial-${index}.${format}`);
      ensureDirFor(partialOut);
      fs.writeFileSync(partialOut, Buffer.from(event.b64_json, 'base64'));
      console.error(`wrote ${partialOut}`);
      finalIndex += 1;
    }
    return;
  }

  const result = await openai.images.generate(request);
  logRequestId(result);

  result.data.forEach((item, index) => {
    if (!item.b64_json) {
      throw new Error('Image response did not include b64_json');
    }
    const filePath =
      n === 1 ? out : out.replace(/(\.[^.]+)?$/, `-${index + 1}.${format}`);
    ensureDirFor(filePath);
    fs.writeFileSync(filePath, Buffer.from(item.b64_json, 'base64'));
    console.log(filePath);
  });
}

main().catch(handleError);

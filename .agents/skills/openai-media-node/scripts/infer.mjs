import {
  createClient,
  handleError,
  logRequestId,
  parseArgs,
  pick,
  requireArg,
} from './media-common.mjs';

async function main() {
  const args = parseArgs();
  const input = requireArg(args, 'input');
  const model = pick(
    args,
    'model',
    process.env.OPENAI_INFERENCE_MODEL || 'gpt-5.5'
  );
  const instructions = pick(args, 'instructions', undefined);

  const request = {
    model,
    input,
    instructions,
  };
  Object.keys(request).forEach((key) => {
    if (request[key] === undefined) delete request[key];
  });

  const openai = createClient('inference');
  const response = await openai.responses.create(request);
  logRequestId(response);
  console.log(response.output_text ?? JSON.stringify(response, null, 2));
}

main().catch(handleError);

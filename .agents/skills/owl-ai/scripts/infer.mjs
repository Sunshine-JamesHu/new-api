import {
  getApiKey,
  getBaseURL,
  handleError,
  loadEnv,
  parseArgs,
  pick,
  requireArg,
  withV1BaseURL,
} from './media-common.mjs';

async function callResponses(args, request) {
  loadEnv();
  const baseURL = withV1BaseURL(pick(args, 'base-url', getBaseURL('inference')));
  if (!baseURL) throw new Error('Missing NEWAPI_BASE_URL or --base-url for the deployed customer site.');
  const response = await fetch(`${baseURL}/responses`, {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${getApiKey('inference')}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(request),
  });
  const text = await response.text();
  let data;
  try {
    data = text ? JSON.parse(text) : {};
  } catch {
    data = { raw: text };
  }
  if (!response.ok) {
    throw new Error(`HTTP ${response.status} ${response.statusText}: ${JSON.stringify(data).slice(0, 1000)}`);
  }
  return data;
}

async function main() {
  const args = parseArgs();
  const input = requireArg(args, 'input');
  const model = pick(
    args,
    'model',
    process.env.INFERENCE_MODEL || 'gpt-5.5'
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

  const response = await callResponses(args, request);
  console.log(response.output_text ?? JSON.stringify(response, null, 2));
}

main()
  .then(() => {
    process.exit(0);
  })
  .catch((error) => {
    handleError(error);
    process.exit(1);
  });

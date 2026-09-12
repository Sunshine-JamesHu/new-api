import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __filename = fileURLToPath(import.meta.url);
export const scriptsDir = path.dirname(__filename);
export const skillDir = path.resolve(scriptsDir, '..');

function findUp(startDir, fileName) {
  let current = path.resolve(startDir);
  while (true) {
    const candidate = path.join(current, fileName);
    if (fs.existsSync(candidate)) return candidate;
    const parent = path.dirname(current);
    if (parent === current) return undefined;
    current = parent;
  }
}

export function loadEnv() {
  const envPaths = [
    path.join(skillDir, '.env'),
    path.join(process.cwd(), '.env'),
    findUp(process.cwd(), '.env'),
    findUp(skillDir, '.env'),
  ];
  for (const envPath of [...new Set(envPaths.filter(Boolean))]) {
    if (!fs.existsSync(envPath)) continue;
    const content = fs.readFileSync(envPath, 'utf8');
    for (const line of content.split(/\r?\n/)) {
      const trimmed = line.trim();
      if (!trimmed || trimmed.startsWith('#')) continue;
      const match = trimmed.match(/^([A-Za-z_][A-Za-z0-9_]*)=(.*)$/);
      if (!match) continue;
      const [, key, rawValue] = match;
      if (process.env[key] !== undefined) continue;
      process.env[key] = rawValue.replace(/^["']|["']$/g, '');
    }
  }
}

export function getApiKey(kind) {
  loadEnv();
  const envNameByKind = {
    image: 'IMAGE_API_KEY',
    video: 'VIDEO_API_KEY',
    inference: 'VIDEO_API_KEY',
  };
  const envName = envNameByKind[kind];
  const value = process.env[envName];
  if (!value) {
    throw new Error(
      `Missing ${envName}. Copy .env.example to .env and fill the key.`
    );
  }
  return value;
}

export function getVideoApiKey({ model, provider } = {}) {
  loadEnv();
  const normalizedModel = String(model || '').toLowerCase();
  const normalizedProvider = String(provider || '').toLowerCase();
  let envName;

  if (normalizedProvider === 'happyhorse' || normalizedModel.includes('happyhorse')) {
    envName = 'HAPPYHORSE_API_KEY';
  } else if (normalizedModel.includes('seedance') || normalizedProvider === 'seedance') {
    envName = 'SEEDANCE_API_KEY';
  } else {
    envName = 'VIDEO_API_KEY';
  }

  const value = process.env[envName];
  if (!value) {
    throw new Error(
      `Missing ${envName}. Copy .env.example to .env and fill the key.`
    );
  }
  return value;
}

export function getBaseURL(kind) {
  loadEnv();
  const envNameByKind = {
    image: 'IMAGE_API_BASE_URL',
    video: 'VIDEO_API_BASE_URL',
    inference: 'VIDEO_API_BASE_URL',
  };
  const envName = envNameByKind[kind];
  return process.env[envName] || process.env.NEWAPI_BASE_URL;
}

export function withV1BaseURL(baseURL) {
  if (!baseURL) return undefined;
  const cleanBaseURL = baseURL.replace(/\/+$/, '');
  return /\/v\d+$/i.test(new URL(cleanBaseURL).pathname)
    ? cleanBaseURL
    : `${cleanBaseURL}/v1`;
}

export function parseArgs(argv = process.argv.slice(2)) {
  const args = {};
  for (let i = 0; i < argv.length; i += 1) {
    const token = argv[i];
    if (!token.startsWith('--')) {
      args._ = [...(args._ || []), token];
      continue;
    }
    const equalIndex = token.indexOf('=');
    if (equalIndex > 2) {
      args[token.slice(2, equalIndex)] = token.slice(equalIndex + 1);
      continue;
    }
    const key = token.slice(2);
    const next = argv[i + 1];
    if (next === undefined || next.startsWith('--')) {
      args[key] = true;
      continue;
    }
    args[key] = next;
    i += 1;
  }
  return args;
}

export function requireArg(args, name) {
  const value = args[name];
  if (value === undefined || value === true || value === '') {
    throw new Error(`Missing required --${name}`);
  }
  return String(value);
}

export function optionalNumber(args, name) {
  const value = args[name];
  if (value === undefined || value === true || value === '') return undefined;
  const parsed = Number(value);
  if (!Number.isFinite(parsed)) throw new Error(`--${name} must be a number`);
  return parsed;
}

export function optionalInteger(args, name) {
  const value = optionalNumber(args, name);
  if (value === undefined) return undefined;
  if (!Number.isInteger(value)) throw new Error(`--${name} must be an integer`);
  return value;
}

export function ensureDirFor(filePath) {
  const dir = path.dirname(path.resolve(filePath));
  fs.mkdirSync(dir, { recursive: true });
}

export function pick(args, key, fallback) {
  const value = args[key];
  if (value === undefined || value === true || value === '') return fallback;
  return String(value);
}

export function parseJsonArg(args, key) {
  const value = args[key];
  if (value === undefined || value === true || value === '') return undefined;
  try {
    return JSON.parse(String(value));
  } catch (error) {
    throw new Error(`--${key} must be valid JSON: ${error.message}`);
  }
}

export function handleError(error) {
  const axiosResponse = error?.response;
  const detail = {
    message: error?.message || error?.code || String(error),
    request_id: error?.request_id,
    status: error?.status,
    code: error?.code,
    type: error?.type,
    moderation_details: error?.moderation_details,
    response_status: axiosResponse?.status,
    response_data: axiosResponse?.data,
  };
  console.error(JSON.stringify(detail, null, 2));
  process.exitCode = 1;
}

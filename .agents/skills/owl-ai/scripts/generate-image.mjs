import fs from "node:fs";
import path from "node:path";
import {
  ensureDirFor,
  getBaseURL,
  getApiKey,
  handleError,
  loadEnv,
  optionalInteger,
  parseArgs,
  pick,
  requireArg,
} from "./media-common.mjs";

function promptFromArgs(args) {
  if (args["prompt-file"]) {
    return fs.readFileSync(path.resolve(String(args["prompt-file"])), "utf8").trim();
  }
  return requireArg(args, "prompt");
}

function saveJson(filePath, value) {
  if (!filePath) return;
  ensureDirFor(filePath);
  fs.writeFileSync(path.resolve(String(filePath)), `${JSON.stringify(value, null, 2)}\n`);
}

function mimeTypeFor(filePath) {
  const lower = String(filePath).toLowerCase();
  if (lower.endsWith(".jpg") || lower.endsWith(".jpeg")) return "image/jpeg";
  if (lower.endsWith(".png")) return "image/png";
  if (lower.endsWith(".webp")) return "image/webp";
  if (lower.endsWith(".gif")) return "image/gif";
  if (lower.endsWith(".bmp")) return "image/bmp";
  if (lower.endsWith(".tif") || lower.endsWith(".tiff")) return "image/tiff";
  if (lower.endsWith(".heic")) return "image/heic";
  if (lower.endsWith(".heif")) return "image/heif";
  return "image/png";
}

function dataUrlFromFile(filePath) {
  const resolved = path.resolve(String(filePath));
  if (!fs.existsSync(resolved)) {
    throw new Error(`Image file not found: ${filePath}`);
  }
  return `data:${mimeTypeFor(resolved)};base64,${fs.readFileSync(resolved).toString("base64")}`;
}

// Accept multi-value flags separated by ';' or ',' (mirrors the video script's
// reference-image list convention). A whole value that is already a data URL is kept
function splitImageList(value) {
  const raw = String(value).trim();
  if (/^data:image\\//i.test(raw)) return [raw];
  return raw.split(/[;,]/).map((part) => part.trim()).filter(Boolean);
}

function collectList(args, names) {
  const values = [];
  for (const name of names) {
    const arg = args[name];
    if (arg === undefined || arg === true || arg === "") continue;
    for (const part of String(arg).split(/[;,]/)) {
      const trimmed = part.trim();
      if (trimmed) values.push(trimmed);
    }
  }
  return values;
}

// Gather reference images like the imagegen skill's multi-image edit: repeated
// --image/--image-files flags translate to local files, while --image-url(s)
// carry existing remote URLs. Local files become data URLs so a single JSON
// request carries all references to the gateway's /v1/images/generations.
function collectReferenceImages(args) {
  const images = [];
  const imagesArg = args["images"] ?? args["reference-images"];
  if (imagesArg !== undefined && imagesArg !== true && imagesArg !== "") {
    for (const part of String(imagesArg).split(/[;,]/)) {
      const trimmed = part.trim();
      if (!trimmed) continue;
      images.push(/^data:image\//i.test(trimmed) || /^https?:\/\//i.test(trimmed)
        ? trimmed
        : dataUrlFromFile(trimmed));
    }
  }
  for (const file of collectList(args, ["image-files", "reference-image-file", "reference-image-files"])) {
    images.push(dataUrlFromFile(file));
  }
  for (const url of collectList(args, ["image-url", "image-urls", "reference-image-url", "reference-image-urls"])) {
    images.push(url);
  }
  const single = args["image"] ?? args["reference-image"];
  if (single !== undefined && single !== true && single !== "") {
    const value = String(single);
    images.push(/^data:image\//i.test(value) || /^https?:\/\//i.test(value)
      ? value
      : dataUrlFromFile(value));
  }
  return [...new Set(images)];
}

function collectBase64Images(value, images = []) {
  if (!value || typeof value !== "object") return images;
  if (Array.isArray(value)) {
    for (const item of value) collectBase64Images(item, images);
    return images;
  }
  for (const [key, item] of Object.entries(value)) {
    const lowerKey = key.toLowerCase();
    if (typeof item === "string" && (lowerKey === "b64_json" || lowerKey.includes("base64"))) {
      images.push(item);
    } else if (item && typeof item === "object") {
      collectBase64Images(item, images);
    }
  }
  return images;
}

function collectImageUrls(value, urls = []) {
  if (!value) return urls;
  if (Array.isArray(value)) {
    for (const item of value) collectImageUrls(item, urls);
    return urls;
  }
  if (typeof value === "object") {
    for (const [key, item] of Object.entries(value)) {
      const lowerKey = key.toLowerCase();
      if (typeof item === "string" && /^https?:\/\//i.test(item)) {
        if (lowerKey.includes("url") || lowerKey.includes("image")) urls.push(item);
      } else {
        collectImageUrls(item, urls);
      }
    }
  }
  return urls;
}

async function downloadUrl(url, filePath) {
  const response = await fetch(url);
  if (!response.ok) throw new Error(`Download failed ${response.status} ${response.statusText}: ${url}`);
  ensureDirFor(filePath);
  fs.writeFileSync(path.resolve(String(filePath)), Buffer.from(await response.arrayBuffer()));
}

async function generateWithFetch(args, request) {
  loadEnv();
  const base = pick(args, "base-url", getBaseURL("image"));
  if (!base) throw new Error("Missing NEWAPI_BASE_URL or --base-url for the deployed customer site.");
  const cleanBase = base.replace(/\/+$/, "");
  const apiPath = /\/v\d+$/i.test(new URL(cleanBase).pathname) ? "/images/generations" : "/v1/images/generations";
  const timeoutMs = optionalInteger(args, "request-timeout") ?? 10 * 60 * 1000;
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), timeoutMs);
  try {
    const response = await fetch(`${cleanBase}${apiPath}`, {
      method: "POST",
      headers: {
        Authorization: `Bearer ${getApiKey("image")}`,
        "Content-Type": "application/json",
      },
      body: JSON.stringify(request),
      signal: controller.signal,
    });
    const text = await response.text();
    let data;
    try {
      data = text ? JSON.parse(text) : {};
    } catch {
      data = { raw: text };
    }
    return { ok: response.ok, status: response.status, statusText: response.statusText, data };
  } finally {
    clearTimeout(timeout);
  }
}

async function main() {
  const args = parseArgs();
  const prompt = promptFromArgs(args);
  const model = pick(args, "model", process.env.IMAGE_MODEL || "gpt-image-2");
  const format = pick(args, "format", "png");
  const out = pick(args, "out", `outputs/image-${Date.now()}.${format}`);
  const n = optionalInteger(args, "n") ?? 1;

  const request = {
    model,
    prompt,
    n,
    size: pick(args, "size", undefined),
    quality: pick(args, "quality", undefined),
    output_format: pick(args, "format", undefined),
    output_compression: optionalInteger(args, "compression"),
    background: pick(args, "background", undefined),
    moderation: pick(args, "moderation", undefined),
  };
  const inputFidelity = pick(args, "input-fidelity", undefined);
  if (inputFidelity !== undefined) request.input_fidelity = inputFidelity;

  const referenceImages = collectReferenceImages(args);
  if (referenceImages.length > 0) {
    request.images = referenceImages;
  } else if (args["image"] !== undefined && args["image"] !== true && args["image"] !== "") {
    request.image = String(args["image"]);
  }

  Object.keys(request).forEach((key) => {
    if (request[key] === undefined) delete request[key];
  });

  saveJson(pick(args, "request-out", undefined), request);
  if (args["dry-run"]) {
    console.log(JSON.stringify(request, null, 2));
    return;
  }

  const fetched = await generateWithFetch(args, request);
  saveJson(pick(args, "response-out", undefined), fetched.data);
  if (!fetched.ok) {
    throw new Error(`HTTP ${fetched.status} ${fetched.statusText}: ${JSON.stringify(fetched.data).slice(0, 1000)}`);
  }
  const b64Images = collectBase64Images(fetched.data);
  if (b64Images.length > 0) {
    b64Images.slice(0, n).forEach((b64, index) => {
      const filePath =
        n === 1 ? out : out.replace(/(\.[^.]+)?$/, `-${index + 1}.${format}`);
      ensureDirFor(filePath);
      fs.writeFileSync(path.resolve(String(filePath)), Buffer.from(b64, "base64"));
      console.log(filePath);
    });
    return;
  }
  const urls = collectImageUrls(fetched.data);
  if (urls.length === 0) throw new Error("Image response did not include image data");
  for (const [index, url] of urls.slice(0, n).entries()) {
    const filePath =
      n === 1 ? out : out.replace(/(\.[^.]+)?$/, `-${index + 1}.${format}`);
    await downloadUrl(url, filePath);
    console.log(filePath);
  }
}

main()
  .then(() => {
    process.exit(0);
  })
  .catch((error) => {
    handleError(error);
    process.exit(1);
  });

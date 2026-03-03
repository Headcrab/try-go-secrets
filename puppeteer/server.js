const express = require("express");
const path = require("path");
const fs = require("fs/promises");
const { constants } = require("fs");
const { spawn } = require("child_process");

const app = express();

const SERVICE_NAME = "puppeteer-render-mvp";
const PORT = Number(process.env.PUPPETEER_PORT || process.env.PORT || 3000);
const HOST = process.env.HOST || "0.0.0.0";
const ROOT_DIR = path.resolve(__dirname, "..");
const STATIC_DIR = path.join(ROOT_DIR, "static");
const TEMPLATES_DIR = path.join(__dirname, "templates");
const DEFAULT_OUTPUT_DIR = path.join(ROOT_DIR, "output", "videos");
const SUPPORTED_TEMPLATES = new Set(["terminal", "diagram"]);

const LIMITS = {
  maxTitleLength: 220,
  maxBlocks: 40,
  maxBlockLength: 14000,
  maxAudioPaths: 64,
  maxScenes: 64,
  maxSceneCaptionLength: 320,
  maxSceneActionLength: 220,
  maxPathLength: 4096,
  minWidth: 360,
  maxWidth: 2160,
  minHeight: 640,
  maxHeight: 3840,
  minFps: 12,
  maxFps: 60,
  minDuration: 1,
  maxDuration: 600,
};

app.use((req, res, next) => {
  const requestId = String(req.headers["x-request-id"] || createRequestId());
  req.renderRequestId = requestId;
  res.setHeader("x-request-id", requestId);

  const startedAt = Date.now();
  res.on("finish", () => {
    logInfo("request_complete", {
      requestId,
      method: req.method,
      path: req.originalUrl,
      statusCode: res.statusCode,
      durationMs: Date.now() - startedAt,
    });
  });
  next();
});

app.use(express.json({ limit: "2mb" }));
app.use("/static", express.static(STATIC_DIR));

function createRequestId() {
  const stamp = Date.now().toString(36);
  const suffix = Math.random().toString(36).slice(2, 8);
  return `${stamp}-${suffix}`;
}

function logInfo(message, meta = {}) {
  console.log(JSON.stringify({ level: "info", ts: new Date().toISOString(), message, ...meta }));
}

function logError(message, meta = {}) {
  console.error(JSON.stringify({ level: "error", ts: new Date().toISOString(), message, ...meta }));
}

function validationError(message, details) {
  const error = new Error(message);
  error.httpStatus = 400;
  error.code = "invalid_request";
  error.details = details;
  return error;
}

function commandError(command, args, message, code, stdout, stderr) {
  const error = new Error(message);
  error.command = command;
  error.args = args;
  error.exitCode = code;
  error.stdout = stdout;
  error.stderr = stderr;
  return error;
}

function sendError(res, httpStatus, status, message, requestId, extra = {}) {
  res.status(httpStatus).json({
    status,
    error: message,
    message,
    requestId,
    ...extra,
  });
}

function isPathInside(targetPath, basePath) {
  const target = path.resolve(targetPath);
  const base = path.resolve(basePath);
  const rel = path.relative(base, target);
  return rel === "" || (!rel.startsWith("..") && !path.isAbsolute(rel));
}

function toTrimmedString(value) {
  return typeof value === "string" ? value.trim() : "";
}

function normalizeIntegerField(value, fieldName, defaultValue, min, max, errors) {
  if (value == null || value === "") {
    return defaultValue;
  }
  const n = Number(value);
  if (!Number.isFinite(n) || !Number.isInteger(n)) {
    errors.push(`${fieldName} must be an integer`);
    return defaultValue;
  }
  if (n < min || n > max) {
    errors.push(`${fieldName} must be between ${min} and ${max}`);
    return defaultValue;
  }
  return n;
}

function normalizeStringArray(value, fieldName, maxItems, maxItemLength, errors) {
  if (value == null) {
    return [];
  }
  if (!Array.isArray(value)) {
    errors.push(`${fieldName} must be an array of strings`);
    return [];
  }
  if (value.length > maxItems) {
    errors.push(`${fieldName} exceeds ${maxItems} items`);
    return [];
  }

  const out = [];
  for (let i = 0; i < value.length; i += 1) {
    const item = value[i];
    if (typeof item !== "string") {
      errors.push(`${fieldName}[${i}] must be a string`);
      continue;
    }
    if (item.length > maxItemLength) {
      errors.push(`${fieldName}[${i}] exceeds ${maxItemLength} characters`);
      continue;
    }
    out.push(item);
  }
  return out;
}

function normalizeMotion(value) {
  const motion = toTrimmedString(value).toLowerCase();
  if (motion === "pan-left" || motion === "pan-right" || motion === "tilt-up") {
    return motion;
  }
  return "push-in";
}

function normalizeScenes(value, errors) {
  if (value == null) {
    return [];
  }
  if (!Array.isArray(value)) {
    errors.push("scenes must be an array");
    return [];
  }
  if (value.length > LIMITS.maxScenes) {
    errors.push(`scenes exceeds ${LIMITS.maxScenes} items`);
    return [];
  }

  const scenes = [];
  for (let i = 0; i < value.length; i += 1) {
    const raw = value[i];
    if (!raw || typeof raw !== "object" || Array.isArray(raw)) {
      errors.push(`scenes[${i}] must be an object`);
      continue;
    }

    const imagePath = toTrimmedString(raw.imagePath);
    if (!imagePath) {
      errors.push(`scenes[${i}].imagePath is required`);
      continue;
    }
    if (imagePath.length > LIMITS.maxPathLength) {
      errors.push(`scenes[${i}].imagePath exceeds ${LIMITS.maxPathLength} characters`);
      continue;
    }

    const caption = toTrimmedString(raw.caption);
    if (caption.length > LIMITS.maxSceneCaptionLength) {
      errors.push(`scenes[${i}].caption exceeds ${LIMITS.maxSceneCaptionLength} characters`);
      continue;
    }

    const action = toTrimmedString(raw.action);
    if (action.length > LIMITS.maxSceneActionLength) {
      errors.push(`scenes[${i}].action exceeds ${LIMITS.maxSceneActionLength} characters`);
      continue;
    }

    let durationSec = Number(raw.durationSec);
    if (!Number.isFinite(durationSec) || durationSec <= 0) {
      durationSec = 4;
    }
    if (durationSec > 30) {
      errors.push(`scenes[${i}].durationSec must be <= 30`);
      continue;
    }

    scenes.push({
      order: Number.isInteger(raw.order) ? raw.order : i + 1,
      caption,
      action,
      durationSec,
      motion: normalizeMotion(raw.motion),
      imagePath,
    });
  }
  return scenes;
}

function normalizeRenderPayload(payload) {
  const errors = [];
  if (!payload || typeof payload !== "object" || Array.isArray(payload)) {
    errors.push("request body must be a JSON object");
    return { ok: false, errors };
  }

  const template = toTrimmedString(payload.template) || "terminal";
  if (!SUPPORTED_TEMPLATES.has(template)) {
    errors.push(`template must be one of: ${[...SUPPORTED_TEMPLATES].join(", ")}`);
  }

  const width = normalizeIntegerField(payload.width, "width", 1080, LIMITS.minWidth, LIMITS.maxWidth, errors);
  const height = normalizeIntegerField(payload.height, "height", 1920, LIMITS.minHeight, LIMITS.maxHeight, errors);
  const fps = normalizeIntegerField(payload.fps, "fps", 30, LIMITS.minFps, LIMITS.maxFps, errors);
  const durationRaw = payload.duration ?? payload.durationSeconds;
  const duration = normalizeIntegerField(
    durationRaw,
    "duration",
    5,
    LIMITS.minDuration,
    LIMITS.maxDuration,
    errors
  );

  let title = "";
  if (payload.title != null) {
    if (typeof payload.title !== "string") {
      errors.push("title must be a string");
    } else {
      title = payload.title.trim();
      if (title.length > LIMITS.maxTitleLength) {
        errors.push(`title exceeds ${LIMITS.maxTitleLength} characters`);
      }
    }
  }

  const codeBlocks = normalizeStringArray(
    payload.codeBlocks,
    "codeBlocks",
    LIMITS.maxBlocks,
    LIMITS.maxBlockLength,
    errors
  );
  const mermaidBlocks = normalizeStringArray(
    payload.mermaidBlocks,
    "mermaidBlocks",
    LIMITS.maxBlocks,
    LIMITS.maxBlockLength,
    errors
  );
  const audioPaths = normalizeStringArray(
    payload.audioPaths,
    "audioPaths",
    LIMITS.maxAudioPaths,
    LIMITS.maxPathLength,
    errors
  );
  const scenes = normalizeScenes(payload.scenes, errors);

  const outputDir = payload.outputDir == null ? "" : toTrimmedString(payload.outputDir);
  if (payload.outputDir != null && !outputDir) {
    errors.push("outputDir must be a non-empty string when provided");
  }

  const outputFileName = payload.outputFileName == null ? "" : toTrimmedString(payload.outputFileName);
  if (payload.outputFileName != null && !outputFileName) {
    errors.push("outputFileName must be a non-empty string when provided");
  }

  const audioMode = toTrimmedString(payload.audioMode).toLowerCase();
  const normalizedAudioMode = audioMode === "mix" ? "mix" : "concat";

  if (errors.length > 0) {
    return { ok: false, errors };
  }

  return {
    ok: true,
    data: {
      template,
      width,
      height,
      fps,
      duration,
      title,
      codeBlocks,
      mermaidBlocks,
      audioPaths,
      scenes,
      outputDir,
      outputFileName,
      audioMode: normalizedAudioMode,
    },
  };
}

function getOutputFileName(template) {
  const stamp = new Date().toISOString().replace(/[-:.TZ]/g, "");
  const suffix = Math.random().toString(36).slice(2, 8);
  return `${template}_${stamp}_${suffix}.mp4`;
}

function sanitizeFileName(fileName, template) {
  const fallback = getOutputFileName(template);
  const raw = toTrimmedString(fileName);
  const baseName = path.basename(raw || fallback);
  const sanitized = baseName
    .replace(/[<>:"/\\|?*\x00-\x1f]/g, "_")
    .replace(/\s+/g, "_")
    .slice(0, 180);

  const withExtension = sanitized.toLowerCase().endsWith(".mp4") ? sanitized : `${sanitized}.mp4`;
  if (!withExtension || withExtension === ".mp4") {
    return fallback;
  }
  return withExtension;
}

function resolveOutputPath(outputDir, outputFileName, template) {
  const explicitOutputDir = Boolean(outputDir);
  const resolvedOutputDir = explicitOutputDir ? path.resolve(outputDir) : DEFAULT_OUTPUT_DIR;
  const safeOutputFileName = sanitizeFileName(outputFileName, template);
  const outputPath = path.resolve(resolvedOutputDir, safeOutputFileName);

  if (!isPathInside(outputPath, resolvedOutputDir)) {
    throw validationError("outputFileName resolves outside outputDir");
  }
  if (!explicitOutputDir && !isPathInside(outputPath, DEFAULT_OUTPUT_DIR)) {
    throw validationError("output path outside default output directory is not allowed");
  }

  return { explicitOutputDir, outputDir: resolvedOutputDir, outputPath, outputFileName: safeOutputFileName };
}

async function ensureDir(dirPath) {
  await fs.mkdir(dirPath, { recursive: true });
}

async function fileReadable(pathname) {
  try {
    await fs.access(pathname, constants.R_OK);
    return true;
  } catch {
    return false;
  }
}

function runCommand(command, args, options = {}) {
  const timeoutMs = options.timeoutMs || 120000;
  return new Promise((resolve, reject) => {
    const child = spawn(command, args, { stdio: ["ignore", "pipe", "pipe"] });
    let stdout = "";
    let stderr = "";
    let timedOut = false;

    const timer = setTimeout(() => {
      timedOut = true;
      child.kill();
    }, timeoutMs);

    child.on("error", (error) => {
      clearTimeout(timer);
      reject(
        commandError(
          command,
          args,
          `failed to start ${command}: ${error.message}`,
          null,
          stdout,
          stderr
        )
      );
    });

    child.stdout.on("data", (chunk) => {
      stdout += chunk.toString();
    });
    child.stderr.on("data", (chunk) => {
      stderr += chunk.toString();
    });

    child.on("close", (code) => {
      clearTimeout(timer);
      if (timedOut) {
        reject(commandError(command, args, `${command} timed out after ${timeoutMs}ms`, code, stdout, stderr));
        return;
      }
      if (code !== 0) {
        reject(
          commandError(
            command,
            args,
            stderr.trim() || `${command} exited with code ${code}`,
            code,
            stdout,
            stderr
          )
        );
        return;
      }
      resolve({ stdout, stderr });
    });
  });
}

async function ffmpegVersion() {
  try {
    const result = await runCommand("ffmpeg", ["-version"], { timeoutMs: 5000 });
    const firstLine = result.stdout.split(/\r?\n/).find(Boolean) || "ok";
    return { available: true, detail: firstLine };
  } catch (error) {
    return { available: false, detail: error.message };
  }
}

async function probeMediaDuration(mediaPath) {
  try {
    const { stdout } = await runCommand(
      "ffprobe",
      [
        "-v",
        "error",
        "-show_entries",
        "format=duration",
        "-of",
        "default=noprint_wrappers=1:nokey=1",
        mediaPath,
      ],
      { timeoutMs: 10000 }
    );
    const value = Number.parseFloat(stdout.trim());
    return Number.isFinite(value) && value > 0 ? value : null;
  } catch {
    return null;
  }
}

async function normalizeAudioPaths(audioPaths) {
  const normalized = [];
  for (let i = 0; i < audioPaths.length; i += 1) {
    const rawPath = audioPaths[i];
    if (/[\r\n]/.test(rawPath)) {
      throw validationError(`audioPaths[${i}] contains invalid control characters`);
    }
    const resolved = path.resolve(rawPath);
    try {
      await fs.access(resolved, constants.R_OK);
      const stat = await fs.stat(resolved);
      if (!stat.isFile()) {
        throw validationError(`audioPaths[${i}] is not a regular file`, { path: resolved });
      }
    } catch (error) {
      if (error.httpStatus) {
        throw error;
      }
      throw validationError(`audioPaths[${i}] is not readable`, { path: resolved });
    }
    normalized.push(resolved);
  }
  return normalized;
}

async function normalizeSceneImagePaths(scenes) {
  const normalized = [];
  for (let i = 0; i < scenes.length; i += 1) {
    const scene = scenes[i];
    const resolved = path.resolve(scene.imagePath);
    try {
      await fs.access(resolved, constants.R_OK);
      const stat = await fs.stat(resolved);
      if (!stat.isFile()) {
        throw validationError(`scenes[${i}].imagePath is not a regular file`, { path: resolved });
      }
    } catch (error) {
      if (error.httpStatus) {
        throw error;
      }
      throw validationError(`scenes[${i}].imagePath is not readable`, { path: resolved });
    }
    normalized.push({
      ...scene,
      imagePath: resolved,
    });
  }
  return normalized;
}

function escapeConcatFilePath(filePath) {
  return filePath.replace(/\\/g, "/").replace(/'/g, "'\\''");
}

function escapeDrawTextValue(value) {
  return value.replace(/\\/g, "\\\\").replace(/:/g, "\\:").replace(/'/g, "\\'").replace(/,/g, "\\,");
}

function normalizeTextLine(value, maxChars) {
  return value.replace(/\t/g, "  ").replace(/\r/g, "").slice(0, maxChars);
}

function toOverlayLines(block, maxLines, maxChars) {
  const lines = block
    .split(/\r?\n/)
    .map((line) => normalizeTextLine(line, maxChars))
    .filter((line) => line.length > 0);
  return lines.slice(0, maxLines);
}

function buildOverlayText(data) {
  const title = data.title || "Go Secrets";
  const lines = [];

  lines.push(`Template: ${data.template}`);
  lines.push(`FPS: ${data.fps}`);
  lines.push(`Requested duration: ${data.duration}s`);

  if (data.codeBlocks.length > 0) {
    lines.push("");
    lines.push("Code preview:");
    lines.push(...toOverlayLines(data.codeBlocks[0], 14, 70));
  }

  if (data.mermaidBlocks.length > 0) {
    lines.push("");
    lines.push("Mermaid preview:");
    lines.push(...toOverlayLines(data.mermaidBlocks[0], 10, 70));
  }

  if (data.codeBlocks.length === 0 && data.mermaidBlocks.length === 0) {
    lines.push("");
    lines.push("No code blocks supplied.");
  }

  return { title, body: lines.slice(0, 30).join("\n") };
}

async function resolveFontFile() {
  const candidates = [
    process.env.FFMPEG_FONT_FILE,
    "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
    "/usr/share/fonts/dejavu/DejaVuSans.ttf",
    "/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf",
  ].filter(Boolean);

  for (const candidate of candidates) {
    const fontPath = path.resolve(candidate);
    // eslint-disable-next-line no-await-in-loop
    if (await fileReadable(fontPath)) {
      return fontPath;
    }
  }
  return "";
}

function drawTextFilter(textFilePath, options) {
  const settings = [];
  if (options.fontFile) {
    settings.push(`fontfile='${escapeDrawTextValue(options.fontFile)}'`);
  }
  settings.push(`textfile='${escapeDrawTextValue(textFilePath)}'`);
  settings.push("reload=1");
  settings.push(`fontcolor=${options.fontColor}`);
  settings.push(`fontsize=${options.fontSize}`);
  settings.push(`line_spacing=${options.lineSpacing}`);
  settings.push(`x=${options.x}`);
  settings.push(`y=${options.y}`);
  settings.push("box=1");
  settings.push(`boxcolor=${options.boxColor}`);
  settings.push(`boxborderw=${options.boxBorder}`);
  return `drawtext=${settings.join(":")}`;
}

function buildVideoFilter(width, height, titleTextPath, bodyTextPath, fontFile) {
  const titleSize = Math.max(42, Math.floor(width * 0.05));
  const bodySize = Math.max(26, Math.floor(width * 0.031));

  const filters = [
    "drawbox=x=0:y=0:w=iw:h=ih:color=0x090F1A:t=fill",
    "drawbox=x=40:y=40:w=iw-80:h=ih-80:color=0x111827@0.88:t=fill",
    "drawbox=x=40:y=40:w=iw-80:h=220:color=0x1F2937@0.95:t=fill",
    "drawgrid=width=80:height=80:thickness=1:color=white@0.04",
    drawTextFilter(titleTextPath, {
      fontFile,
      fontColor: "0xF9FAFB",
      fontSize: titleSize,
      lineSpacing: 8,
      x: 72,
      y: 108,
      boxColor: "0x000000@0.35",
      boxBorder: 18,
    }),
    drawTextFilter(bodyTextPath, {
      fontFile,
      fontColor: "0xE5E7EB",
      fontSize: bodySize,
      lineSpacing: 12,
      x: 72,
      y: 320,
      boxColor: "0x000000@0.2",
      boxBorder: 20,
    }),
  ];

  return filters.join(",");
}

async function prepareAudioTrack(audioPaths, audioMode, tempDir, requestId) {
  if (audioPaths.length === 0) {
    return { audioPath: "", audioDuration: null };
  }
  if (audioPaths.length === 1) {
    const audioDuration = await probeMediaDuration(audioPaths[0]);
    return { audioPath: audioPaths[0], audioDuration };
  }

  if (audioMode === "mix") {
    const mixedPath = path.join(tempDir, `audio-mixed-${requestId}.m4a`);
    const args = ["-y"];
    for (const audioPath of audioPaths) {
      args.push("-i", audioPath);
    }
    const amixInputs = audioPaths.map((_, i) => `[${i}:a]`).join("");
    args.push(
      "-filter_complex",
      `${amixInputs}amix=inputs=${audioPaths.length}:dropout_transition=0:normalize=0[aout]`,
      "-map",
      "[aout]",
      "-ac",
      "2",
      "-ar",
      "48000",
      "-c:a",
      "aac",
      "-b:a",
      "192k",
      mixedPath
    );
    await runCommand("ffmpeg", args, { timeoutMs: 120000 });
    const audioDuration = await probeMediaDuration(mixedPath);
    return { audioPath: mixedPath, audioDuration };
  }

  const concatListPath = path.join(tempDir, `audio-list-${requestId}.txt`);
  const mergedAudioPath = path.join(tempDir, `audio-merged-${requestId}.m4a`);
  const concatList = `${audioPaths.map((audioPath) => `file '${escapeConcatFilePath(audioPath)}'`).join("\n")}\n`;
  await fs.writeFile(concatListPath, concatList, "utf8");

  await runCommand(
    "ffmpeg",
    [
      "-y",
      "-f",
      "concat",
      "-safe",
      "0",
      "-i",
      concatListPath,
      "-vn",
      "-ac",
      "2",
      "-ar",
      "48000",
      "-c:a",
      "aac",
      "-b:a",
      "192k",
      mergedAudioPath,
    ],
    { timeoutMs: 180000 }
  );

  const audioDuration = await probeMediaDuration(mergedAudioPath);
  return { audioPath: mergedAudioPath, audioDuration };
}

function buildFfmpegArgs(data, outputPath, titleTextPath, bodyTextPath, audioPath, audioDuration, fontFile) {
  const sourceDuration = audioPath
    ? Number.isFinite(audioDuration)
      ? Math.max(data.duration, Math.ceil(audioDuration + 1))
      : Math.max(data.duration, 600)
    : data.duration;

  const filter = buildVideoFilter(data.width, data.height, titleTextPath, bodyTextPath, fontFile);
  const args = [
    "-y",
    "-f",
    "lavfi",
    "-i",
    `color=c=#0B1220:s=${data.width}x${data.height}:r=${data.fps}:d=${sourceDuration}`,
  ];

  if (audioPath) {
    args.push("-i", audioPath);
  }

  args.push(
    "-vf",
    filter,
    "-r",
    String(data.fps),
    "-c:v",
    "libx264",
    "-preset",
    "medium",
    "-crf",
    "22",
    "-pix_fmt",
    "yuv420p",
    "-movflags",
    "+faststart"
  );

  if (audioPath) {
    args.push(
      "-map",
      "0:v:0",
      "-map",
      "1:a:0",
      "-c:a",
      "aac",
      "-b:a",
      "192k",
      "-ar",
      "48000",
      "-ac",
      "2",
      "-shortest"
    );
  } else {
    args.push("-an");
  }

  args.push(outputPath);
  return { args, sourceDuration };
}

function buildFallbackFfmpegArgs(data, outputPath, audioPath, audioDuration) {
  const sourceDuration = audioPath
    ? Number.isFinite(audioDuration)
      ? Math.max(data.duration, Math.ceil(audioDuration + 1))
      : Math.max(data.duration, 600)
    : data.duration;

  const args = [
    "-y",
    "-f",
    "lavfi",
    "-i",
    `color=c=#0B1220:s=${data.width}x${data.height}:r=${data.fps}:d=${sourceDuration}`,
  ];

  if (audioPath) {
    args.push("-i", audioPath);
  }

  args.push(
    "-r",
    String(data.fps),
    "-c:v",
    "libx264",
    "-preset",
    "medium",
    "-crf",
    "22",
    "-pix_fmt",
    "yuv420p",
    "-movflags",
    "+faststart"
  );

  if (audioPath) {
    args.push(
      "-map",
      "0:v:0",
      "-map",
      "1:a:0",
      "-c:a",
      "aac",
      "-b:a",
      "192k",
      "-ar",
      "48000",
      "-ac",
      "2",
      "-shortest"
    );
  } else {
    args.push("-an");
  }

  args.push(outputPath);
  return { args, sourceDuration };
}

function wrapCaption(text, maxLineLength = 42, maxLines = 3) {
  const compact = String(text || "")
    .replace(/\s+/g, " ")
    .trim();
  if (!compact) {
    return "";
  }
  const words = compact.split(" ");
  const lines = [];
  let current = "";
  for (const word of words) {
    if (current.length === 0) {
      current = word;
      continue;
    }
    if ((current + word).length + 1 <= maxLineLength) {
      current += ` ${word}`;
      continue;
    }
    lines.push(current);
    current = word;
    if (lines.length >= maxLines) {
      break;
    }
  }
  if (lines.length < maxLines && current) {
    lines.push(current);
  }
  return lines.slice(0, maxLines).join("\n");
}

function buildSceneMotionFilter(scene, width, height) {
  const duration = Math.max(1, Number(scene.durationSec) || 4);
  const durationExpr = duration.toFixed(3);
  const overscanWidth = Math.round(width * 1.25);
  const overscanHeight = Math.round(height * 1.25);
  const base = `scale=${overscanWidth}:${overscanHeight}:force_original_aspect_ratio=increase`;

  switch (scene.motion) {
    case "pan-left":
      return `${base},crop=${width}:${height}:x='(in_w-out_w)*(t/${durationExpr})':y='(in_h-out_h)/2':eval=frame`;
    case "pan-right":
      return `${base},crop=${width}:${height}:x='(in_w-out_w)*(1-(t/${durationExpr}))':y='(in_h-out_h)/2':eval=frame`;
    case "tilt-up":
      return `${base},crop=${width}:${height}:x='(in_w-out_w)/2':y='(in_h-out_h)*(1-(t/${durationExpr}))':eval=frame`;
    default:
      return `${base},crop='in_w-(in_w*0.10*(t/${durationExpr}))':'in_h-(in_h*0.10*(t/${durationExpr}))':x='(in_w-out_w)/2':y='(in_h-out_h)/2':eval=frame,scale=${width}:${height}`;
  }
}

function buildSceneClipFilter(scene, data, captionFilePath, fontFile, staticMotion = false) {
  const motionFilter = staticMotion
    ? `scale=${Math.round(data.width * 1.18)}:${Math.round(data.height * 1.18)}:force_original_aspect_ratio=increase,crop=${data.width}:${data.height}:x='(in_w-out_w)/2':y='(in_h-out_h)/2'`
    : buildSceneMotionFilter(scene, data.width, data.height);
  const filters = [motionFilter];
  filters.push("eq=saturation=1.08:contrast=1.03");
  filters.push("fade=t=in:st=0:d=0.35");
  const fadeOutStart = Math.max(0, scene.durationSec - 0.35).toFixed(3);
  filters.push(`fade=t=out:st=${fadeOutStart}:d=0.35`);

  if (captionFilePath) {
    filters.push(
      drawTextFilter(captionFilePath, {
        fontFile,
        fontColor: "0xF9FAFB",
        fontSize: Math.max(34, Math.floor(data.width * 0.034)),
        lineSpacing: 10,
        x: "(w-text_w)/2",
        y: "h-280",
        boxColor: "0x000000@0.48",
        boxBorder: 20,
      })
    );
  }
  return filters.join(",");
}

function buildSceneClipArgs(scene, data, clipPath, captionFilePath, fontFile, staticMotion = false) {
  const duration = Math.max(1, Number(scene.durationSec) || 4);
  const filter = buildSceneClipFilter(scene, data, captionFilePath, fontFile, staticMotion);
  return [
    "-y",
    "-loop",
    "1",
    "-i",
    scene.imagePath,
    "-t",
    duration.toFixed(3),
    "-vf",
    filter,
    "-r",
    String(data.fps),
    "-c:v",
    "libx264",
    "-preset",
    "medium",
    "-crf",
    "22",
    "-pix_fmt",
    "yuv420p",
    "-an",
    clipPath,
  ];
}

function isSceneMotionFailure(error) {
  const stderr = String(error?.stderr || "");
  const message = String(error?.message || "");
  const payload = `${message}\n${stderr}`.toLowerCase();
  return (
    payload.includes("parsed_crop") ||
    payload.includes("error when evaluating the expression") ||
    payload.includes("failed to configure input pad on parsed_crop")
  );
}

async function renderSceneVideoTrack(scenes, data, tempDir, requestId, fontFile) {
  const clipPaths = [];
  for (let i = 0; i < scenes.length; i += 1) {
    const scene = scenes[i];
    const clipPath = path.join(tempDir, `scene-${String(i + 1).padStart(2, "0")}.mp4`);
    const wrappedCaption = wrapCaption(scene.caption || scene.action);
    const captionPath = wrappedCaption && fontFile
      ? path.join(tempDir, `scene-${String(i + 1).padStart(2, "0")}-caption.txt`)
      : "";
    if (captionPath) {
      // eslint-disable-next-line no-await-in-loop
      await fs.writeFile(captionPath, `${wrappedCaption}\n`, "utf8");
    }
    const args = buildSceneClipArgs(scene, data, clipPath, captionPath, fontFile, false);
    // eslint-disable-next-line no-await-in-loop
    try {
      await runCommand("ffmpeg", args, { timeoutMs: 4 * 60 * 1000 });
    } catch (error) {
      if (!isSceneMotionFailure(error)) {
        throw error;
      }
      const fallbackArgs = buildSceneClipArgs(scene, data, clipPath, captionPath, fontFile, true);
      // eslint-disable-next-line no-await-in-loop
      await runCommand("ffmpeg", fallbackArgs, { timeoutMs: 4 * 60 * 1000 });
    }
    clipPaths.push(clipPath);
  }

  if (clipPaths.length === 1) {
    return clipPaths[0];
  }

  const concatListPath = path.join(tempDir, `scene-list-${requestId}.txt`);
  const concatList = `${clipPaths.map((clipPath) => `file '${escapeConcatFilePath(clipPath)}'`).join("\n")}\n`;
  await fs.writeFile(concatListPath, concatList, "utf8");

  const mergedPath = path.join(tempDir, `scene-track-${requestId}.mp4`);
  await runCommand(
    "ffmpeg",
    [
      "-y",
      "-f",
      "concat",
      "-safe",
      "0",
      "-i",
      concatListPath,
      "-c:v",
      "libx264",
      "-preset",
      "medium",
      "-crf",
      "22",
      "-pix_fmt",
      "yuv420p",
      "-r",
      String(data.fps),
      "-an",
      mergedPath,
    ],
    { timeoutMs: 4 * 60 * 1000 }
  );
  return mergedPath;
}

function buildMuxArgs(videoPath, outputPath, data, audioPath) {
  const args = ["-y", "-i", videoPath];
  if (audioPath) {
    args.push("-i", audioPath);
  }

  args.push(
    "-c:v",
    "libx264",
    "-preset",
    "medium",
    "-crf",
    "22",
    "-pix_fmt",
    "yuv420p",
    "-movflags",
    "+faststart"
  );

  if (audioPath) {
    args.push(
      "-map",
      "0:v:0",
      "-map",
      "1:a:0",
      "-c:a",
      "aac",
      "-b:a",
      "192k",
      "-ar",
      "48000",
      "-ac",
      "2",
      "-shortest"
    );
  } else {
    args.push("-an");
  }

  args.push("-r", String(data.fps), outputPath);
  return args;
}

function isDrawTextFailure(error) {
  const stderr = String(error?.stderr || "");
  const message = String(error?.message || "");
  const payload = `${message}\n${stderr}`.toLowerCase();
  return (
    payload.includes("fontconfig error") ||
    payload.includes("cannot load default config file") ||
    payload.includes("no such filter: 'drawtext'") ||
    payload.includes("error initializing filter 'drawtext'")
  );
}

app.get("/health", async (_req, res) => {
  const ffmpeg = await ffmpegVersion();
  res.json({
    status: "ok",
    service: SERVICE_NAME,
    ffmpegAvailable: ffmpeg.available,
    ffmpegDetail: ffmpeg.detail,
    templates: [...SUPPORTED_TEMPLATES],
    outputDir: DEFAULT_OUTPUT_DIR,
  });
});

app.get("/templates/:name", async (req, res) => {
  const template = req.params.name;
  if (!SUPPORTED_TEMPLATES.has(template)) {
    sendError(res, 404, "not_found", "template not found", req.renderRequestId, {
      supported: [...SUPPORTED_TEMPLATES],
    });
    return;
  }

  const templatePath = path.join(TEMPLATES_DIR, `${template}.html`);
  if (!(await fileReadable(templatePath))) {
    sendError(res, 404, "not_found", "template file missing", req.renderRequestId, { template });
    return;
  }

  res.sendFile(templatePath);
});

app.post("/render", async (req, res) => {
  const requestId = req.renderRequestId || createRequestId();
  const normalized = normalizeRenderPayload(req.body || {});
  if (!normalized.ok) {
    sendError(res, 400, "invalid_request", "request validation failed", requestId, {
      validationErrors: normalized.errors,
      supportedTemplates: [...SUPPORTED_TEMPLATES],
    });
    return;
  }

  const ffmpeg = await ffmpegVersion();
  if (!ffmpeg.available) {
    sendError(res, 503, "unavailable", "ffmpeg is not available; render skipped", requestId, {
      ffmpegDetail: ffmpeg.detail,
      fallback: "Install ffmpeg and make sure it is in PATH, then retry /render",
    });
    return;
  }

  const data = normalized.data;
  let tempDir = "";

  try {
    const output = resolveOutputPath(data.outputDir, data.outputFileName, data.template);
    await ensureDir(output.outputDir);
    tempDir = path.join(output.outputDir, `.render-tmp-${requestId}`);
    await ensureDir(tempDir);

    const safeAudioPaths = await normalizeAudioPaths(data.audioPaths);
    const safeScenes = await normalizeSceneImagePaths(data.scenes);
    const fontFile = await resolveFontFile();
    const audio = await prepareAudioTrack(safeAudioPaths, data.audioMode, tempDir, requestId);

    logInfo("render_started", {
      requestId,
      template: data.template,
      outputPath: output.outputPath,
      width: data.width,
      height: data.height,
      fps: data.fps,
      requestedDuration: data.duration,
      audioCount: safeAudioPaths.length,
      sceneCount: safeScenes.length,
      audioMode: data.audioMode,
    });

    let usedOverlay = false;
    if (safeScenes.length > 0) {
      const sceneTrackPath = await renderSceneVideoTrack(safeScenes, data, tempDir, requestId, fontFile);
      const muxArgs = buildMuxArgs(sceneTrackPath, output.outputPath, data, audio.audioPath);
      await runCommand("ffmpeg", muxArgs, { timeoutMs: 8 * 60 * 1000 });
      usedOverlay = Boolean(fontFile);
    } else {
      const overlayText = buildOverlayText(data);
      const titleTextPath = path.join(tempDir, "title.txt");
      const bodyTextPath = path.join(tempDir, "body.txt");
      await fs.writeFile(titleTextPath, `${overlayText.title}\n`, "utf8");
      await fs.writeFile(bodyTextPath, `${overlayText.body}\n`, "utf8");
      const { args } = buildFfmpegArgs(
        data,
        output.outputPath,
        titleTextPath,
        bodyTextPath,
        audio.audioPath,
        audio.audioDuration,
        fontFile
      );

      usedOverlay = true;
      try {
        await runCommand("ffmpeg", args, { timeoutMs: 8 * 60 * 1000 });
      } catch (error) {
        if (!isDrawTextFailure(error)) {
          throw error;
        }
        usedOverlay = false;
        logInfo("render_retry_without_drawtext", {
          requestId,
          reason: "drawtext_or_fontconfig_unavailable",
        });
        const fallback = buildFallbackFfmpegArgs(data, output.outputPath, audio.audioPath, audio.audioDuration);
        await runCommand("ffmpeg", fallback.args, { timeoutMs: 8 * 60 * 1000 });
      }
    }

    const outputDuration = await probeMediaDuration(output.outputPath);
    const finalDuration = Number.isFinite(outputDuration)
      ? outputDuration
      : Number.isFinite(audio.audioDuration)
        ? audio.audioDuration
        : data.duration;

    logInfo("render_completed", {
      requestId,
      outputPath: output.outputPath,
      finalDurationSeconds: finalDuration,
    });

    res.status(201).json({
      status: "rendered",
      template: data.template,
      outputPath: output.outputPath,
      duration: Number(finalDuration.toFixed(2)),
      durationSeconds: Number(finalDuration.toFixed(2)),
      width: data.width,
      height: data.height,
      fps: data.fps,
      audioCount: safeAudioPaths.length,
      sceneCount: safeScenes.length,
      overlayEnabled: usedOverlay,
      requestId,
    });
  } catch (error) {
    const httpStatus = error.httpStatus || 500;
    const status = error.code || (httpStatus === 400 ? "invalid_request" : "failed");
    logError("render_failed", {
      requestId,
      status,
      httpStatus,
      message: error.message,
      stderr: error.stderr ? error.stderr.slice(-3000) : undefined,
      details: error.details,
    });

    sendError(res, httpStatus, status, error.message || "render failed", requestId, {
      details: error.details,
      ffmpegStderr: error.stderr ? error.stderr.slice(-3000) : undefined,
    });
  } finally {
    if (tempDir) {
      await fs.rm(tempDir, { recursive: true, force: true }).catch(() => {});
    }
  }
});

app.use((error, req, res, next) => {
  if (error && error.type === "entity.parse.failed") {
    sendError(res, 400, "invalid_request", "invalid JSON payload", req.renderRequestId || createRequestId());
    return;
  }
  if (error) {
    const requestId = req.renderRequestId || createRequestId();
    logError("unhandled_error", { requestId, message: error.message });
    sendError(res, 500, "failed", "internal server error", requestId);
    return;
  }
  next();
});

function startServer(port = PORT, host = HOST) {
  return new Promise((resolve) => {
    const server = app.listen(port, host, () => {
      const address = server.address();
      const resolvedPort = typeof address === "object" && address ? address.port : port;
      console.log(`puppeteer service listening on http://${host}:${resolvedPort}`);
      resolve(server);
    });
  });
}

if (require.main === module) {
  startServer().catch((error) => {
    console.error(`failed to start service: ${error.message}`);
    process.exit(1);
  });
}

module.exports = { app, startServer };

import pageClient from "./page-client.js";
import { page } from "./page.js";
import wasmModule from "../generated/gencrc.wasm";
import "../generated/wasm_exec.js";

const MAX_UPLOAD_BYTES = 64 * 1024 * 1024;
const CORS_HEADERS = {
  "access-control-allow-origin": "*",
  "access-control-allow-methods": "GET, POST, OPTIONS",
  "access-control-allow-headers": "content-type",
  "access-control-max-age": "86400",
};
const GAMES = {
  generals: { version: "1.08", iniName: "INI.big" },
  generalsmd: { version: "1.04", iniName: "INIZH.big" },
};

let ready;

function runtime() {
  if (!ready) {
    const go = new globalThis.Go();
    ready = WebAssembly.instantiate(wasmModule, go.importObject).then(instance => {
      void go.run(instance);
      if (typeof globalThis.genCRCCalculate !== "function" ||
          typeof globalThis.genCRCInspectExecutable !== "function") {
        throw new Error("Wasm runtime failed to initialize");
      }
    });
  }
  return ready;
}

async function asset(env, game, name) {
  const response = await env.RETAIL.fetch(new Request(`https://retail/${game}/${name}`));
  if (!response.ok) throw new Error(`retail snapshot is incomplete: ${game}/${name}`);
  return new Uint8Array(await response.arrayBuffer());
}

function apiResponse(value, status = 200) {
  return Response.json(value, {
    status,
    headers: { ...CORS_HEADERS, "cache-control": "no-store" },
  });
}

function apiError(code, message, status) {
  return apiResponse({ error: { code, message } }, status);
}

class UploadError extends Error {
  constructor(code, message, status = 400) {
    super(message);
    this.code = code;
    this.status = status;
  }
}

function safePath(name) {
  const value = name.replaceAll("\\", "/").replace(/^\/+/, "");
  if (!value || value.split("/").includes("..")) {
    throw new Error(`unsafe upload path: ${name}`);
  }
  return value;
}

function parseJSON(value, fallback) {
  if (value === null) return fallback;
  if (typeof value !== "string") throw new Error("manifest must be text");
  return JSON.parse(value);
}

function resolveSource(source, files, offset, label) {
  if (!source || !["big", "folder"].includes(source.kind) ||
      !Array.isArray(source.files) || source.files.length === 0) {
    throw new Error(`invalid ${label} source`);
  }
  if (source.kind === "big" && source.files.length !== 1) {
    throw new Error(`${label} BIG source must contain one file`);
  }
  return {
    kind: source.kind,
    name: String(source.name || ""),
    files: source.files.map(ref => {
      if (!Number.isInteger(ref.index) || ref.index < 0 || ref.index >= files.length) {
        throw new Error(`${label} manifest references an invalid file`);
      }
      const name = safePath(String(ref.name || files[ref.index].name));
      if (source.kind === "big" && !name.toLowerCase().endsWith(".big")) {
        throw new Error(`${label} BIG must have a .big filename`);
      }
      return { name, blob: offset + ref.index };
    }),
  };
}

function resolveManifest(form, additionalFiles, modFiles) {
  let additionalRaw, modRaw;
  try {
    additionalRaw = parseJSON(form.get("additional_manifest"), []);
    modRaw = parseJSON(form.get("mod_manifest"), null);
  } catch {
    throw new UploadError("invalid_manifest", "Upload manifest is not valid JSON");
  }
  if (!Array.isArray(additionalRaw)) {
    throw new UploadError("invalid_manifest", "additional_manifest must be an array");
  }

  let additional, mod;
  try {
    additional = additionalRaw.map(source => resolveSource(source, additionalFiles, 0, "Additional Files"));
    mod = modRaw === null ? null : resolveSource(modRaw, modFiles, additionalFiles.length, "Mod Files");
  } catch (error) {
    throw new UploadError("invalid_manifest", error.message);
  }
  if (additionalFiles.length && !additional.length) {
    throw new UploadError("invalid_manifest", "additional_file requires additional_manifest");
  }
  if (modFiles.length && !mod) {
    throw new UploadError("invalid_manifest", "mod_file requires mod_manifest");
  }
  return { additional, mod };
}

async function readUpload(request) {
  if (Number(request.headers.get("content-length") || 0) > MAX_UPLOAD_BYTES) {
    throw new UploadError("upload_too_large", "Upload exceeds 64 MiB", 413);
  }
  const form = await request.formData();
  const exeFile = form.get("exe");
  const nonemptyFiles = name => form.getAll(name).filter(value => value instanceof File && value.size);
  const additionalFiles = nonemptyFiles("additional_file");
  const modFiles = nonemptyFiles("mod_file");
  const exe = exeFile instanceof File && exeFile.size
    ? new Uint8Array(await exeFile.arrayBuffer()) : new Uint8Array();
  if (!exe.length && !additionalFiles.length && !modFiles.length) {
    throw new UploadError("missing_input", "Provide an executable, Additional Files, or Mod Files");
  }
  const files = [...additionalFiles, ...modFiles];
  const total = files.reduce((size, file) => size + file.size, exe.length);
  if (total > MAX_UPLOAD_BYTES) {
    throw new UploadError("upload_too_large", "Combined files exceed 64 MiB", 413);
  }
  const manifest = resolveManifest(form, additionalFiles, modFiles);
  const blobs = await Promise.all(files.map(async file => new Uint8Array(await file.arrayBuffer())));
  return { selectedGame: form.get("game"), exe, manifest, blobs };
}

async function checksumResponse(request, env) {
  try {
    const { selectedGame, exe, manifest, blobs } = await readUpload(request);
    await runtime();
    const inspected = exe.length ? JSON.parse(globalThis.genCRCInspectExecutable(exe)) : null;
    if (inspected?.error) {
      return apiError(inspected.error_code || "unsupported_executable", inspected.error, 422);
    }
    const game = inspected ? inspected.game : selectedGame;
    if (!Object.hasOwn(GAMES, game)) {
      return apiError("game_required", exe.length
        ? "Could not detect game from executable"
        : "Game is required when no executable is uploaded", 422);
    }

    const { iniName } = GAMES[game];
    const needINI = manifest.additional.length || manifest.mod;
    const [ini, skirmish, multiplayer] = await Promise.all([
      needINI ? asset(env, game, iniName) : new Uint8Array(),
      exe.length ? asset(env, game, "SkirmishScripts.scb") : new Uint8Array(),
      exe.length ? asset(env, game, "MultiplayerScripts.scb") : new Uint8Array(),
    ]);
    const result = JSON.parse(globalThis.genCRCCalculate(
      game, exe, JSON.stringify(manifest), blobs, iniName, ini, skirmish, multiplayer,
    ));
    if (result.error) return apiError(result.error_code || "checksum_failed", result.error, 422);
    return apiResponse(result);
  } catch (error) {
    if (error instanceof UploadError) return apiError(error.code, error.message, error.status);
    return apiError("checksum_failed", error instanceof Error ? error.message : "Checksum failed", 422);
  }
}

async function retailResponse(url, env) {
  const game = url.searchParams.get("game");
  if (!Object.hasOwn(GAMES, game)) {
    return apiError("game_required", "Choose generals (1.08) or generalsmd (1.04)", 422);
  }
  try {
    await runtime();
    const { iniName, version } = GAMES[game];
    const ini = await asset(env, game, iniName);
    const empty = new Uint8Array();
    const result = JSON.parse(globalThis.genCRCCalculate(
      game, empty, JSON.stringify({ additional: [], mod: null }), [], iniName, ini, empty, empty,
    ));
    if (result.error || !result.ini_crc) {
      return apiError("checksum_failed", result.error || "Retail INI checksum is unavailable", 422);
    }
    return apiResponse({ game, version, ini_crc: result.ini_crc });
  } catch (error) {
    return apiError("checksum_failed", error instanceof Error ? error.message : "Retail checksum failed", 422);
  }
}

function apiDiscovery() {
  return apiResponse({
    service: "GenCRC",
    endpoint: "/api/checksum",
    method: "POST",
    content_type: "multipart/form-data",
    retail: {
      endpoint: "/api/retail",
      method: "GET",
      query: { game: Object.keys(GAMES) },
      versions: { generals: GAMES.generals.version, generalsmd: GAMES.generalsmd.version },
      fields: ["game", "version", "ini_crc"],
    },
    modes: ["retail", "exe-only", "additional-only", "mod-only", "combined"],
    games: Object.keys(GAMES),
    inputs: {
      game: {
        type: "string", required: "when exe is absent", values: Object.keys(GAMES),
        ignored: "when exe is present",
      },
      exe: { type: "file", required: false },
      additional_manifest: {
        type: "json", schema: "ordered array of {kind: big|folder, name?, files: [{name,index}]}",
      },
      additional_file: { type: "file", repeatable: true, indexed_by: "additional_manifest" },
      mod_manifest: { type: "json", schema: "one {kind: big|folder, name?, files: [{name,index}]}" },
      mod_file: { type: "file", repeatable: true, indexed_by: "mod_manifest" },
    },
    constraints: {
      at_least_one: ["exe", "additional_file", "mod_file"],
      maximum_combined_bytes: MAX_UPLOAD_BYTES,
    },
  });
}

export default {
  async fetch(request, env) {
    const url = new URL(request.url);
    if (request.method === "GET") {
      switch (url.pathname) {
        case "/":
          return new Response(page, { headers: { "content-type": "text/html; charset=utf-8" } });
        case "/page-client.js":
          return new Response(pageClient, {
            headers: { "content-type": "text/javascript; charset=utf-8", "cache-control": "no-cache" },
          });
        case "/logo.png":
        case "/favicon.ico":
          // Never forward the public URL: the binding also contains private retail inputs.
          return env.RETAIL.fetch(new Request("https://retail/logo.png"));
        case "/api":
          return apiDiscovery();
        case "/api/retail":
          return retailResponse(url, env);
      }
    }
    if (request.method === "OPTIONS" && ["/api/checksum", "/api/retail"].includes(url.pathname)) {
      return new Response(null, { status: 204, headers: CORS_HEADERS });
    }
    if (request.method === "POST" && url.pathname === "/api/checksum") {
      return checksumResponse(request, env);
    }
    // Paired with assets.run_worker_first: there is deliberately no asset fallback.
    return apiError("not_found", "Endpoint not found", 404);
  },
};

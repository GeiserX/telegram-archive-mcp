#!/usr/bin/env node
"use strict";

// Downloads the release binary for this platform and checks it against the
// checksums.txt bundled into the npm package at publish time. The bundled copy
// is covered by npm's provenance, so a release asset replaced after publishing
// is refused instead of run.
//
// npm 12 skips install scripts by default, so run.js calls install() on first
// start when the binary is missing. Every log line goes to stderr: under run.js,
// stdout is the MCP channel.

const { execFileSync } = require("child_process");
const crypto = require("crypto");
const fs = require("fs");
const https = require("https");
const os = require("os");
const path = require("path");

const VERSION = require("./package.json").version;
const REPO = "GeiserX/telegram-archive-mcp";
const BIN_NAME = process.platform === "win32" ? "telegram-archive-mcp.exe" : "telegram-archive-mcp";
const BIN_DIR = path.join(__dirname, "bin");
const BIN_PATH = path.join(BIN_DIR, BIN_NAME);
const CHECKSUMS_PATH = path.join(__dirname, "checksums.txt");

const MAX_REDIRECTS = 5;
const REQUEST_TIMEOUT_MS = 30_000;
// The archives are about 5 MB; anything far larger is not one of them.
const MAX_DOWNLOAD_BYTES = 64 * 1024 * 1024;
// GitHub serves release assets from github.com, then redirects to its asset host.
const ALLOWED_HOSTS = new Set([
  "github.com",
  "release-assets.githubusercontent.com",
  "objects.githubusercontent.com",
]);

function log(msg) {
  process.stderr.write(`${msg}\n`);
}

function getPlatformArch() {
  const platformMap = { darwin: "darwin", linux: "linux", win32: "windows" };
  const archMap = { x64: "amd64", arm64: "arm64" };
  const p = platformMap[process.platform];
  const a = archMap[process.arch];
  if (!p || !a) {
    throw new Error(`Unsupported platform: ${process.platform}-${process.arch}`);
  }
  return { platform: p, arch: a };
}

function getAssetName() {
  const { platform, arch } = getPlatformArch();
  const ext = platform === "windows" ? "zip" : "tar.gz";
  return `telegram-archive-mcp_${VERSION}_${platform}_${arch}.${ext}`;
}

// checkURL refuses anything but https to a GitHub release host, so a redirect
// can neither downgrade to cleartext nor point somewhere else.
function checkURL(raw, base) {
  const u = new URL(raw, base);
  if (u.protocol !== "https:") {
    throw new Error(`Refusing non-https download URL: ${u.protocol}//${u.host}`);
  }
  if (!ALLOWED_HOSTS.has(u.hostname)) {
    throw new Error(`Refusing download from unexpected host: ${u.hostname}`);
  }
  return u;
}

// expectedHash returns the sha256 for assetName from checksums.txt content.
function expectedHash(checksumsText, assetName) {
  for (const line of checksumsText.split(/\r?\n/)) {
    const m = line.trim().match(/^([0-9a-f]{64})\s+\*?(.+)$/);
    if (m && m[2] === assetName) {
      return m[1];
    }
  }
  throw new Error(`No checksum for ${assetName} in the bundled checksums.txt`);
}

function readBundledChecksums() {
  try {
    return fs.readFileSync(CHECKSUMS_PATH, "utf-8");
  } catch {
    throw new Error(
      "checksums.txt is not bundled with this package, so the download cannot be verified. Install a published release from npm."
    );
  }
}

// download streams url to dest, following at most MAX_REDIRECTS redirects to
// allowed hosts, and returns the sha256 of what it wrote.
function download(url, dest, redirects = 0) {
  return new Promise((resolve, reject) => {
    let u;
    try {
      u = checkURL(url);
    } catch (err) {
      return reject(err);
    }
    const req = https.get(u, { timeout: REQUEST_TIMEOUT_MS }, (res) => {
      if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
        res.resume();
        if (redirects >= MAX_REDIRECTS) {
          return reject(new Error(`Too many redirects (>${MAX_REDIRECTS})`));
        }
        let next;
        try {
          next = new URL(res.headers.location, u).toString();
        } catch (err) {
          return reject(err);
        }
        return download(next, dest, redirects + 1).then(resolve, reject);
      }
      if (res.statusCode !== 200) {
        res.resume();
        return reject(new Error(`Download failed: HTTP ${res.statusCode}`));
      }
      const hash = crypto.createHash("sha256");
      const out = fs.createWriteStream(dest, { mode: 0o600 });
      let size = 0;
      res.on("data", (chunk) => {
        size += chunk.length;
        if (size > MAX_DOWNLOAD_BYTES) {
          req.destroy(new Error(`Download larger than ${MAX_DOWNLOAD_BYTES} bytes`));
          return;
        }
        hash.update(chunk);
      });
      res.on("error", reject);
      out.on("error", reject);
      out.on("finish", () => resolve(hash.digest("hex")));
      res.pipe(out);
    });
    req.on("timeout", () => req.destroy(new Error(`Request timed out after ${REQUEST_TIMEOUT_MS}ms`)));
    req.on("error", reject);
  });
}

async function install() {
  if (fs.existsSync(BIN_PATH)) {
    return;
  }
  const assetName = getAssetName();
  const want = expectedHash(readBundledChecksums(), assetName);
  const url = `https://github.com/${REPO}/releases/download/v${VERSION}/${assetName}`;

  fs.mkdirSync(BIN_DIR, { recursive: true });
  const work = fs.mkdtempSync(path.join(os.tmpdir(), "telegram-archive-mcp-"));
  try {
    const archive = path.join(work, assetName);
    log(`Downloading telegram-archive-mcp v${VERSION} for ${process.platform}-${process.arch}...`);
    const got = await download(url, archive);
    if (got !== want) {
      throw new Error(`Checksum mismatch for ${assetName}: expected ${want}, got ${got}`);
    }
    log("SHA-256 checksum verified against the bundled checksums.txt.");

    // tar ships with macOS, Linux and Windows 10+, and bsdtar (macOS, Windows)
    // reads zip too. Arguments go straight to the program, never through a shell.
    execFileSync("tar", ["-xf", archive, "-C", work, BIN_NAME], { stdio: "ignore" });
    const extracted = path.join(work, BIN_NAME);
    if (process.platform !== "win32") {
      fs.chmodSync(extracted, 0o755);
    }
    // Move into place in one step, so a concurrent start never sees half a file.
    const staged = `${BIN_PATH}.${process.pid}.tmp`;
    fs.copyFileSync(extracted, staged);
    fs.renameSync(staged, BIN_PATH);
    log(`Installed telegram-archive-mcp to ${BIN_PATH}`);
  } finally {
    fs.rmSync(work, { recursive: true, force: true });
  }
}

module.exports = { install, checkURL, expectedHash, getAssetName, ALLOWED_HOSTS, BIN_PATH };

if (require.main === module) {
  install().catch((err) => {
    log(`Failed to install telegram-archive-mcp: ${err.message}`);
    process.exit(1);
  });
}

#!/usr/bin/env node
"use strict";

const { execSync } = require("child_process");
const fs = require("fs");
const path = require("path");
const https = require("https");
const http = require("http");

const VERSION = require("./package.json").version;
const REPO = "GeiserX/telegram-archive-mcp";
const BIN_NAME = process.platform === "win32" ? "telegram-archive-mcp.exe" : "telegram-archive-mcp";
const BIN_DIR = path.join(__dirname, "bin");
const BIN_PATH = path.join(BIN_DIR, BIN_NAME);

function getPlatformArch() {
  const platform = process.platform;
  const arch = process.arch;

  const platformMap = { darwin: "darwin", linux: "linux", win32: "windows" };
  const archMap = { x64: "amd64", arm64: "arm64" };

  const p = platformMap[platform];
  const a = archMap[arch];

  if (!p || !a) {
    throw new Error(`Unsupported platform: ${platform}-${arch}`);
  }

  return { platform: p, arch: a };
}

function getAssetName() {
  const { platform, arch } = getPlatformArch();
  const ext = platform === "windows" ? "zip" : "tar.gz";
  return `telegram-archive-mcp_${VERSION}_${platform}_${arch}.${ext}`;
}

// NOTE: No checksum verification is performed on the downloaded binary.
// The download relies on HTTPS transport security from GitHub Releases.
const MAX_REDIRECTS = 10;
const REQUEST_TIMEOUT_MS = 30_000;

function downloadFile(url, redirectCount = 0) {
  return new Promise((resolve, reject) => {
    if (redirectCount > MAX_REDIRECTS) {
      return reject(new Error(`Too many redirects (>${MAX_REDIRECTS})`));
    }
    const get = url.startsWith("https") ? https.get : http.get;
    const req = get(url, { timeout: REQUEST_TIMEOUT_MS }, (res) => {
      if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
        return downloadFile(res.headers.location, redirectCount + 1).then(resolve, reject);
      }
      if (res.statusCode !== 200) {
        return reject(new Error(`Download failed: HTTP ${res.statusCode}`));
      }
      const chunks = [];
      res.on("data", (chunk) => chunks.push(chunk));
      res.on("end", () => resolve(Buffer.concat(chunks)));
      res.on("error", reject);
    });
    req.on("timeout", () => {
      req.destroy();
      reject(new Error(`Request timed out after ${REQUEST_TIMEOUT_MS}ms`));
    });
    req.on("error", reject);
  });
}

async function extract(buffer, assetName) {
  fs.mkdirSync(BIN_DIR, { recursive: true });

  if (assetName.endsWith(".zip")) {
    // .zip extraction: use PowerShell on Windows, unzip on Unix
    const tmpZip = path.join(BIN_DIR, "tmp.zip");
    fs.writeFileSync(tmpZip, buffer);
    if (process.platform === "win32") {
      execSync(
        `powershell -NoProfile -Command "Expand-Archive -Force -Path '${tmpZip}' -DestinationPath '${BIN_DIR}'"`,
        { stdio: "ignore" }
      );
    } else {
      execSync(`unzip -o "${tmpZip}" "${BIN_NAME}" -d "${BIN_DIR}"`, { stdio: "ignore" });
    }
    fs.unlinkSync(tmpZip);
  } else {
    const tmpTar = path.join(BIN_DIR, "tmp.tar.gz");
    fs.writeFileSync(tmpTar, buffer);
    execSync(`tar -xzf "${tmpTar}" -C "${BIN_DIR}" "${BIN_NAME}"`, { stdio: "ignore" });
    fs.unlinkSync(tmpTar);
  }

  if (process.platform !== "win32") {
    fs.chmodSync(BIN_PATH, 0o755);
  }
}

async function main() {
  if (fs.existsSync(BIN_PATH)) {
    console.log("telegram-archive-mcp binary already exists, skipping download.");
    return;
  }

  const assetName = getAssetName();
  const url = `https://github.com/${REPO}/releases/download/v${VERSION}/${assetName}`;

  console.log(`Downloading telegram-archive-mcp v${VERSION} for ${process.platform}-${process.arch}...`);
  const buffer = await downloadFile(url);

  console.log("Extracting binary...");
  await extract(buffer, assetName);

  console.log(`Installed telegram-archive-mcp to ${BIN_PATH}`);
}

main().catch((err) => {
  console.error(`Failed to install telegram-archive-mcp: ${err.message}`);
  process.exit(1);
});

"use strict";

const test = require("node:test");
const assert = require("node:assert");
const { checkURL, expectedHash, getAssetName } = require("./postinstall.js");

const SUMS = [
  "70cc8115c8c917d6957b09a2ae6b2d344d14645881f2c5470cb09a05bcf84bac  telegram-archive-mcp_0.3.1_darwin_amd64.tar.gz",
  "9f0ea782f10c85468644b6b8adef4602fb712905583dea08d9fd5c54ad55625c  telegram-archive-mcp_0.3.1_windows_amd64.zip",
  "",
].join("\n");

test("checkURL accepts https GitHub release hosts", () => {
  for (const u of [
    "https://github.com/GeiserX/telegram-archive-mcp/releases/download/v1/x.tar.gz",
    "https://release-assets.githubusercontent.com/github-production-release-asset/1/2",
    "https://objects.githubusercontent.com/github-production-release-asset-2e65be/1",
  ]) {
    assert.strictEqual(checkURL(u).toString(), u);
  }
});

test("checkURL refuses a downgrade to http", () => {
  assert.throws(() => checkURL("http://github.com/GeiserX/x"), /non-https/);
  assert.throws(() => checkURL("http://release-assets.githubusercontent.com/x"), /non-https/);
});

test("checkURL refuses any other host", () => {
  assert.throws(() => checkURL("https://evil.example/x.tar.gz"), /unexpected host/);
  assert.throws(() => checkURL("https://github.com.evil.example/x"), /unexpected host/);
});

test("expectedHash finds the asset's line", () => {
  assert.strictEqual(
    expectedHash(SUMS, "telegram-archive-mcp_0.3.1_windows_amd64.zip"),
    "9f0ea782f10c85468644b6b8adef4602fb712905583dea08d9fd5c54ad55625c"
  );
});

test("expectedHash refuses an asset with no line, and a suffix match", () => {
  assert.throws(() => expectedHash(SUMS, "telegram-archive-mcp_0.3.1_linux_arm64.tar.gz"), /No checksum/);
  assert.throws(() => expectedHash(SUMS, "amd64.zip"), /No checksum/);
});

test("getAssetName names this version's archive", () => {
  const version = require("./package.json").version;
  assert.match(getAssetName(), new RegExp(`^telegram-archive-mcp_${version.replace(/\./g, "\\.")}_(darwin|linux|windows)_(amd64|arm64)\\.(tar\\.gz|zip)$`));
});

import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync, mkdirSync, readFileSync, writeFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { createHash } from 'node:crypto';
import { assets, preparePackages } from './prepare-packages.mjs';

function fixture(t) {
  const root = mkdtempSync(path.join(tmpdir(), 'kexoz-packages-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const directory = path.join(root, 'assets');
  mkdirSync(directory);
  const manifest = assets.map((name) => {
    const binary = Buffer.from(`fixture ${name}`);
    writeFileSync(path.join(directory, name), binary);
    return `${createHash('sha256').update(binary).digest('hex')}  ${name}`;
  }).join('\n');
  writeFileSync(path.join(directory, 'SHA256SUMS'), manifest);
  return { directory, output: path.join(root, 'packages'), manifest };
}

test('generates versioned Homebrew and portable Chocolatey packages from all six verified assets', (t) => {
  const { directory, output } = fixture(t);
  const { formula, checksums } = preparePackages('v1.2.3', directory, output);
  assert.match(formula, /version "1\.2\.3"/);
  assert.match(formula, /license :cannot_represent/);
  for (const name of assets.filter((name) => !name.endsWith('.exe'))) {
    assert.ok(formula.includes(`/v1.2.3/${name}"\n        sha256 "${checksums.get(name)}"`));
  }
  const install = readFileSync(path.join(output, 'chocolatey/tools/chocolateyInstall.ps1'), 'utf8');
  assert.match(install, /Get-ChocolateyWebFile/);
  assert.doesNotMatch(install, /Install-ChocolateyPackage|PLACEHOLDER|@KEXOZ_/);
  assert.ok(install.includes(checksums.get('kexoz-windows-amd64.exe')));
  assert.ok(install.includes(checksums.get('kexoz-windows-arm64.exe')));
  assert.ok(install.includes("$version = '1.2.3'"));
});

test('refuses corrupt, incomplete, duplicate-checksum and unsafe-tag releases', (t) => {
  const { directory, output, manifest } = fixture(t);
  assert.throws(() => preparePackages('v1.2.3;echo bad', directory, output), /stable/);
  writeFileSync(path.join(directory, 'kexoz-linux-amd64'), 'tampered');
  assert.throws(() => preparePackages('v1.2.3', directory, output), /Invalid release asset/);
  writeFileSync(path.join(directory, 'SHA256SUMS'), manifest.split('\n').slice(1).join('\n'));
  assert.throws(() => preparePackages('v1.2.3', directory, output), /six/);
  writeFileSync(path.join(directory, 'SHA256SUMS'), `${manifest}\n${manifest.split('\n')[0]}`);
  assert.throws(() => preparePackages('v1.2.3', directory, output), /duplicate/);
});

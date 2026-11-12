import { createHash } from 'node:crypto';
import { readFileSync, writeFileSync, mkdirSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const root = fileURLToPath(new URL('../', import.meta.url));
export const assets = ['windows', 'linux', 'darwin'].flatMap((os) =>
  ['amd64', 'arm64'].map((arch) => `kexoz-${os}-${arch}${os === 'windows' ? '.exe' : ''}`));

export function verifyRelease(directory) {
  const entries = new Map();
  for (const line of readFileSync(path.join(directory, 'SHA256SUMS'), 'utf8').trim().split(/\r?\n/)) {
    const match = line.match(/^([a-fA-F0-9]{64})\s+\*?(\S+)\s*$/);
    if (!match || entries.has(match[2])) throw new Error('Invalid or duplicate checksum entry');
    entries.set(match[2], match[1].toLowerCase());
  }
  if (entries.size !== assets.length) throw new Error('Expected exactly six release checksums');
  for (const name of assets) {
    const binary = readFileSync(path.join(directory, name));
    const actual = createHash('sha256').update(binary).digest('hex');
    if (!binary.length || entries.get(name) !== actual) throw new Error(`Invalid release asset: ${name}`);
  }
  return entries;
}

export function preparePackages(tag, directory, output) {
  if (!/^v\d+\.\d+\.\d+$/.test(tag)) throw new Error('Use a stable vX.Y.Z tag');
  const checksums = verifyRelease(directory);
  const version = tag.slice(1);
  const url = (name) => `https://github.com/wrenyn/kexoz-cli/releases/download/${tag}/${name}`;
  const stanza = (os, arch) => {
    const name = `kexoz-${os}-${arch}`;
    return `        url "${url(name)}"\n        sha256 "${checksums.get(name)}"`;
  };
  const formula = `# Generated from verified GitHub release assets. Do not edit checksums manually.
class Kexoz < Formula
  desc "Share local web apps and TCP/UDP services from your terminal"
  homepage "https://kexoz.dev"
  version "${version}"
  license :cannot_represent

  on_macos do
    on_arm do
${stanza('darwin', 'arm64')}
    end
    on_intel do
${stanza('darwin', 'amd64')}
    end
  end
  on_linux do
    on_arm do
${stanza('linux', 'arm64')}
    end
    on_intel do
${stanza('linux', 'amd64')}
    end
  end

  def install
    bin.install "kexoz-#{OS.mac? ? 'darwin' : 'linux'}-#{Hardware::CPU.arm? ? 'arm64' : 'amd64'}" => "kexoz"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/kexoz --version")
  end
end
`;
  const homebrewDir = path.join(output, 'homebrew', 'Formula');
  const chocoDir = path.join(output, 'chocolatey');
  mkdirSync(homebrewDir, { recursive: true });
  mkdirSync(path.join(chocoDir, 'tools'), { recursive: true });
  writeFileSync(path.join(homebrewDir, 'kexoz.rb'), formula);
  for (const name of ['kexoz.nuspec', 'tools/chocolateyInstall.ps1', 'tools/chocolateyUninstall.ps1']) {
    const content = readFileSync(path.join(root, 'packages/chocolatey', name), 'utf8')
      .replaceAll('@KEXOZ_VERSION@', version)
      .replaceAll('@KEXOZ_SHA256_AMD64@', checksums.get('kexoz-windows-amd64.exe'))
      .replaceAll('@KEXOZ_SHA256_ARM64@', checksums.get('kexoz-windows-arm64.exe'));
    if (/@KEXOZ_/.test(content)) throw new Error(`Unresolved package template: ${name}`);
    writeFileSync(path.join(chocoDir, name), content);
  }
  return { formula, checksums };
}

if (process.argv[1] && import.meta.url === pathToFileURL(path.resolve(process.argv[1])).href) {
  try {
    const [tag, directory, output] = process.argv.slice(2);
    if (!tag || !directory || !output) throw new Error('Usage: node scripts/prepare-packages.mjs vX.Y.Z assets-dir output-dir');
    preparePackages(tag, directory, output);
    console.log(`Prepared Homebrew and Chocolatey packages for ${tag}`);
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}

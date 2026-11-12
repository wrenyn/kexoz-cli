const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const os = require('node:os');
const path = require('node:path');
const { createHash } = require('node:crypto');
const { spawnSync } = require('node:child_process');
const { assetName, expectedChecksum, install } = require('../install');

test('maps exactly the six released platform binaries', () => {
    for (const [platform, goos] of Object.entries({ win32: 'windows', darwin: 'darwin', linux: 'linux' })) {
        for (const [arch, goarch] of Object.entries({ x64: 'amd64', arm64: 'arm64' })) {
            assert.equal(assetName(platform, arch), `kexoz-${goos}-${goarch}${platform === 'win32' ? '.exe' : ''}`);
        }
    }
    assert.throws(() => assetName('linux', 'ia32'), /Unsupported/);
    assert.throws(() => assetName('freebsd', 'x64'), /Unsupported/);
});

test('requires an unambiguous checksum for the exact asset', () => {
    const hash = 'a'.repeat(64);
    assert.equal(expectedChecksum(Buffer.from(`${hash}  kexoz-linux-amd64\r\n`), 'kexoz-linux-amd64'), hash);
    assert.throws(() => expectedChecksum(Buffer.from(`${hash}  kexoz-linux-arm64`), 'kexoz-linux-amd64'), /Missing/);
    assert.throws(() => expectedChecksum(Buffer.from(`${hash}  x\n${hash}  x`), 'x'), /duplicate/);
});

test('installs a verified, pinned binary with executable permissions', async (t) => {
    const binDir = await fs.mkdtemp(path.join(os.tmpdir(), 'kexoz-npm-'));
    t.after(() => fs.rm(binDir, { recursive: true, force: true }));
    const binary = Buffer.from('verified CLI fixture');
    const hash = createHash('sha256').update(binary).digest('hex');
    const urls = [];
    const destination = await install({ platform: 'linux', arch: 'x64', version: '1.2.3', binDir,
        fetchAsset: async (url) => {
            urls.push(url);
            return url.endsWith('/SHA256SUMS') ? Buffer.from(`${hash}  kexoz-linux-amd64\n`) : binary;
        },
    });
    assert.deepEqual(urls, ['SHA256SUMS', 'kexoz-linux-amd64'].map((name) => `https://github.com/wrenyn/kexoz-cli/releases/download/v1.2.3/${name}`));
    assert.deepEqual(await fs.readFile(destination), binary);
    if (process.platform !== 'win32') assert.equal((await fs.stat(destination)).mode & 0o777, 0o755);
});

test('failed verification or download preserves an existing installation', async (t) => {
    const binDir = await fs.mkdtemp(path.join(os.tmpdir(), 'kexoz-npm-'));
    t.after(() => fs.rm(binDir, { recursive: true, force: true }));
    const destination = path.join(binDir, 'kexoz.exe');
    await fs.writeFile(destination, 'previous binary');
    for (const failure of ['checksum', 'network']) {
        await assert.rejects(install({ platform: 'win32', arch: 'arm64', binDir,
            fetchAsset: async (url) => {
                if (failure === 'network') throw new Error('network unavailable');
                return url.endsWith('/SHA256SUMS') ? Buffer.from(`${'0'.repeat(64)}  kexoz-windows-arm64.exe`) : Buffer.from('corrupted');
            },
        }), /Checksum mismatch|network unavailable/);
        assert.equal(await fs.readFile(destination, 'utf8'), 'previous binary');
        assert.deepEqual(await fs.readdir(binDir), ['kexoz.exe']);
    }
});

test('wrapper passes arguments and exit status to the native CLI', (t) => {
    if (!process.env.KEXOZ_TEST_BINARY) return t.skip('CI supplies a native Go build');
    const wrapper = path.join(__dirname, '../bin/kexoz.js');
    const result = spawnSync(process.execPath, [wrapper, '--version'], { encoding: 'utf8' });
    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /^kexoz /);
    const invalid = spawnSync(process.execPath, [wrapper, 'not-a-command'], { encoding: 'utf8' });
    assert.equal(invalid.status, 1);
});

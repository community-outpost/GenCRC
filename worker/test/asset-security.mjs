import assert from 'node:assert/strict';
import { mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { dirname, join, relative } from 'node:path';
import { fileURLToPath } from 'node:url';
import { createTestHarness, unstable_readConfig } from 'wrangler';
import { baselineArchive } from './fixtures.js';

const workerRoot = fileURLToPath(new URL('../', import.meta.url));
const generated = join(workerRoot, 'generated');
mkdirSync(generated, { recursive: true });
const fixture = mkdtempSync(join(generated, 'asset-security-'));
const production = unstable_readConfig({ config: join(workerRoot, 'wrangler.jsonc') });
const config = {
  name: production.name,
  main: production.main,
  compatibility_date: production.compatibility_date,
  compatibility_flags: production.compatibility_flags,
  rules: production.rules,
  assets: { ...production.assets, directory: fixture },
};

const logo = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=', 'base64');
const privatePaths = [
  '/generals/INI.big',
  '/generals/SkirmishScripts.scb',
  '/generals/MultiplayerScripts.scb',
  '/generalsmd/INIZH.big',
  '/generalsmd/SkirmishScripts.scb',
  '/generalsmd/MultiplayerScripts.scb',
  '/manifest.json',
];
for (const path of privatePaths) {
  const destination = join(fixture, path.slice(1));
  mkdirSync(dirname(destination), { recursive: true });
  writeFileSync(destination, path.endsWith('.big') ? baselineArchive() : 'private-synthetic-fixture');
}
writeFileSync(join(fixture, 'logo.png'), logo);

// The negative control proves requests pass through the actual assets router:
// calling the Worker fetch handler directly would return404 in both cases.
const server = createTestHarness({
  root: workerRoot,
  workers: [{ config: { ...config, assets: { ...config.assets, run_worker_first: false } } }],
});

try {
  await server.listen();
  const leaked = await server.fetch('/generals/INI.big');
  assert.equal(leaked.status, 200, 'negative control must reproduce asset-first bypass');
  assert.deepEqual(new Uint8Array(await leaked.arrayBuffer()), baselineArchive());

  // Use the actual repository setting, not a hardcoded true in the test fixture.
  await server.update({ root: workerRoot, workers: [{ config }] });
  const privateVariants = [
    ...privatePaths,
    '/generals/INI.big?download=1',
    '/%67enerals/INI.big',
    '/generals/INI%2ebig',
    '/generals%2fINI.big',
    '/generals%252fINI.big',
    '/generals/./INI.big',
    '/logo.png/../generals/INI.big',
    '/logo.png/%2e%2e/generals/INI.big',
    '/generals\\INI.big',
    '/generals/',
    '/retail/generals/INI.big',
  ];
  for (const path of privateVariants) {
    for (const method of ['GET', 'HEAD']) {
      const response = await server.fetch(path, { method, redirect: 'manual' });
      assert.equal(response.status, 404, `${method} ${path} must not expose retail files`);
      if (method === 'GET') {
        assert.deepEqual(await response.json(), { error: { code: 'not_found', message: 'Endpoint not found' } });
      }
    }
  }
  const rangeResponse = await server.fetch('/generals/INI.big', { headers: { Range: 'bytes=0-3' } });
  assert.equal(rangeResponse.status, 404);
  await rangeResponse.arrayBuffer();

  for (const path of ['/logo.png', '/favicon.ico', '/logo.png?file=/generals/INI.big']) {
    const response = await server.fetch(path);
    assert.equal(response.status, 200);
    assert.deepEqual(Buffer.from(await response.arrayBuffer()), logo);
  }
  assert.match(await (await server.fetch('/')).text(), /Gen/);
  assert.equal(await (await server.fetch('/page-client.js')).text(), readFileSync(join(workerRoot, 'src/page-client.js'), 'utf8'));
  assert.equal((await (await server.fetch('/api')).json()).service, 'GenCRC');

  for (const [game, version] of [['generals', '1.08'], ['generalsmd', '1.04']]) {
    const response = await server.fetch('/api/retail?game=' + game);
    assert.equal(response.status, 200);
    assert.deepEqual(await response.json(), { game, version, ini_crc: '41000000' });
  }
  const form = new FormData();
  form.set('exe', new File(['Launcher config file missing\0launcher.cfg'], 'Generals.exe'));
  // Native fetch serializes Node's FormData; the harness dispatch API expects its own body types.
  const { url } = await server.listen();
  const rejected = await fetch(new URL('/api/checksum', url), { method: 'POST', body: form });
  assert.equal(rejected.status, 422);
  const rejection = await rejected.json();
  assert.equal(rejection.error.code, 'launcher_executable', JSON.stringify(rejection));
  console.log('Asset routing security checks passed');
} catch (error) {
  server.debug();
  throw error;
} finally {
  await server.close();
  // Only remove this invocation's generated fixture, never the real retail directory.
  assert.match(relative(generated, fixture), /^asset-security-[^/\\]+$/);
  rmSync(fixture, { recursive: true, force: true });
}

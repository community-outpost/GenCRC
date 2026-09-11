import { test, expect } from 'bun:test';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { runInNewContext } from 'node:vm';
import { page } from './page.js';
import { Element } from '../test/dom-harness.js';

// Build with the actual deployment bundler: source-only tests cannot catch
// helpers injected into a function that is later serialized into HTML.
test('Wrangler delivers standalone browser code and working picker handlers', async () => {
  const worker = fileURLToPath(new URL('../', import.meta.url));
  const output = join(worker, 'generated/client-delivery-test');
  const build = Bun.spawnSync([
    process.execPath, 'node_modules/wrangler/bin/wrangler.js', 'deploy',
    '--dry-run', '--outdir', output,
  ], { cwd: worker, env: { ...process.env, WRANGLER_LOG_PATH: join(worker, 'generated/wrangler-test.log') } });
  expect(build.exitCode).toBe(0);
  const bundle = readFileSync(join(output, 'index.js'), 'utf8');
  const emitted = bundle.match(/from "\.\/([^"/]+-page-client\.js)"/)?.[1];
  expect(emitted).toBeDefined();
  const source = readFileSync(join(output, emitted), 'utf8');
  expect(source).toBe(readFileSync(new URL('./page-client.js', import.meta.url), 'utf8'));
  expect(page).toContain('import { initializePage } from "/page-client.js"; initializePage();');
  expect(bundle).toContain(emitted);
  expect(bundle).toContain('"/page-client.js"');

  const elements = Object.fromEntries([...page.matchAll(/id="([^"]+)"/g)].map(match => [match[1], new Element()]));
  elements.form.elements = { game: elements.game };
  elements.game.value = 'generalsmd';
  // Only remove the ESM export declaration for the VM, preserving every
  // delivered function byte and running without Worker/bundler globals.
  runInNewContext(source.replace('export function initializePage', 'function initializePage') + '\ninitializePage();', {
    document: { getElementById: id => elements[id], createElement: () => new Element() },
  });
  for (const kind of ['exe', 'additional', 'mod']) {
    await elements[kind + '-file-link'].click();
    expect(elements[kind + '-picker'].clicks).toBe(1);
    if (kind !== 'exe') {
      await elements[kind + '-folder-link'].click();
      expect(elements[kind + '-folder-picker'].clicks).toBe(1);
    }
  }
}, 30000);

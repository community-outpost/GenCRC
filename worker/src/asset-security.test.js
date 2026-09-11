import { test, expect } from 'bun:test';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';

test('Wrangler asset routing keeps retail private while APIs can read it', async () => {
  const worker = fileURLToPath(new URL('../', import.meta.url));
  // Wrangler's integration harness is a Node API and owns a real workerd process.
  const child = Bun.spawn(['node', 'test/asset-security.mjs'], {
    cwd: worker,
    env: {
      ...process.env,
      WRANGLER_SEND_METRICS: 'false',
      WRANGLER_LOG_PATH: join(worker, 'generated/asset-security.log'),
    },
    stdout: 'pipe',
    stderr: 'pipe',
  });
  const [exitCode, stdout, stderr] = await Promise.all([
    child.exited, new Response(child.stdout).text(), new Response(child.stderr).text(),
  ]);
  expect({ exitCode, output: exitCode ? stdout + stderr : '' }).toEqual({ exitCode: 0, output: '' });
  expect(stdout).toContain('Asset routing security checks passed');
}, 60000);

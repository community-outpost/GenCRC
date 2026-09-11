import {test, expect} from 'bun:test';
import {readFileSync} from 'node:fs';
import {runInNewContext} from 'node:vm';

function endpoint(calculate = () => JSON.stringify({ini_crc: 'FEAAE3F3'}), inspect = () => JSON.stringify({game: 'generalsmd'})) {
  const source = readFileSync(new URL('./index.js', import.meta.url), 'utf8')
    .replace(/^import .*;\r?\n/gm, '')
    .replace(/export default\s*\{/, 'globalThis.worker = {');
  const context = {
    URL, Request, Response, Uint8Array, File, page: '', pageClient: '', wasmModule: {},
    Go: class {importObject = {}; run() {}},
    WebAssembly: {instantiate: async () => ({})},
    genCRCCalculate: calculate, genCRCInspectExecutable: inspect,
  };
  runInNewContext(source, context);
  return context.worker;
}
const request = (path, method = 'GET') => new Request('https://example.test' + path, {method});

test('executable rejection preserves actionable codes before accessing snapshots', async () => {
  for (const code of ['launcher_executable', 'unsupported_executable']) {
    let calculated = false;
    const error = {error_code: code, error: 'Upload Game.dat, not a launcher.'};
    const worker = endpoint(() => {calculated = true;}, () => JSON.stringify(error));
    const body = new FormData();
    body.append('exe', new File(['MZ'], 'Generals.exe'));
    const response = await worker.fetch(new Request('https://example.test/api/checksum', {method: 'POST', body}), {});
    expect(response.status).toBe(422);
    expect(await response.json()).toEqual({error: {code, message: error.error}});
    expect(calculated).toBe(false);
  }
});

test('retail validates game before accessing assets and shares CORS/no-store contract', async () => {
  const worker = endpoint();
  for (const query of ['', '?game=unknown', '?game=__proto__', '?game=constructor']) {
    const response = await worker.fetch(request('/api/retail' + query), {});
    expect(response.status).toBe(422);
    expect((await response.json()).error.code).toBe('game_required');
    expect(response.headers.get('access-control-allow-origin')).toBe('*');
    expect(response.headers.get('cache-control')).toBe('no-store');
  }
  expect((await worker.fetch(request('/api/retail', 'OPTIONS'), {})).status).toBe(204);
  expect((await worker.fetch(request('/api/retail', 'POST'), {})).status).toBe(404);
});

test('retail sends only bundled INI to Go and attaches the correct release version', async () => {
  for (const [game, version, iniName] of [['generals', '1.08', 'INI.big'], ['generalsmd', '1.04', 'INIZH.big']]) {
    let args, assetURL;
    const worker = endpoint((...values) => {args = values; return JSON.stringify({ini_crc: '12345678'});});
    const response = await worker.fetch(request('/api/retail?game=' + game), {
      RETAIL: {fetch: async input => {assetURL = input.url; return new Response(new Uint8Array([1, 2, 3]));}},
    });
    expect(await response.json()).toEqual({game, version, ini_crc: '12345678'});
    expect(assetURL).toBe('https://retail/' + game + '/' + iniName);
    expect(args[0]).toBe(game);
    expect(args[1]).toHaveLength(0);
    expect(JSON.parse(args[2])).toEqual({additional: [], mod: null});
    expect(args[3]).toHaveLength(0);
    expect(args[4]).toBe(iniName);
    expect([...args[5]]).toEqual([1, 2, 3]);
    expect(args[6]).toHaveLength(0);
    expect(args[7]).toHaveLength(0);
  }
});

test('retail reports missing snapshots and Go failures without fabricating checksums', async () => {
  const env = {RETAIL: {fetch: async () => new Response('missing', {status: 404})}};
  expect((await endpoint().fetch(request('/api/retail?game=generals'), env)).status).toBe(422);
  for (const body of [{error: 'Invalid BIG'}, {}]) {
    const response = await endpoint(() => JSON.stringify(body)).fetch(request('/api/retail?game=generals'), {
      RETAIL: {fetch: async () => new Response(new Uint8Array([1]))},
    });
    expect(response.status).toBe(422);
    expect((await response.json()).error.code).toBe('checksum_failed');
  }
});

test('checksum infers EXE, INI, and combined work without changing manifest order', async () => {
  for (const [hasExe, hasAdditional, hasMod] of [
    [true, false, false], [false, true, false], [false, false, true], [true, true, true],
  ]) {
    let args;
    const assets = [];
    const worker = endpoint((...values) => {
      args = values;
      return JSON.stringify({ game: values[0], ...(hasExe && { exe_crc: '12345678' }), ...((hasAdditional || hasMod) && { ini_crc: 'ABCDEF01' }) });
    });
    const form = new FormData();
    form.set('game', hasExe ? 'ignored-for-executable' : 'generalsmd');
    if (hasExe) form.set('exe', new File(['exe'], 'Game.dat'));
    if (hasAdditional) {
      form.append('additional_file', new File(['first'], 'z.big'));
      form.append('additional_file', new File(['second'], 'a.big'));
      form.set('additional_manifest', JSON.stringify([
        { kind: 'big', name: 'z.big', files: [{ name: 'z.big', index: 0 }] },
        { kind: 'big', name: 'a.big', files: [{ name: 'a.big', index: 1 }] },
      ]));
    }
    if (hasMod) {
      form.append('mod_file', new File(['mod'], 'mod.big'));
      form.set('mod_manifest', JSON.stringify({ kind: 'folder', name: 'MyMod', files: [{ name: 'mod.big', index: 0 }] }));
    }
    const response = await worker.fetch(new Request('https://example.test/api/checksum', { method: 'POST', body: form }), {
      RETAIL: { fetch: async input => { assets.push(new URL(input.url).pathname); return new Response('fixture'); } },
    });
    expect(response.status).toBe(200);
    const result = await response.json();
    expect(result.game).toBe('generalsmd');
    expect(Boolean(result.exe_crc)).toBe(hasExe);
    expect(Boolean(result.ini_crc)).toBe(hasAdditional || hasMod);
    expect(assets.sort()).toEqual([
      ...((hasAdditional || hasMod) ? ['/generalsmd/INIZH.big'] : []),
      ...(hasExe ? ['/generalsmd/SkirmishScripts.scb', '/generalsmd/MultiplayerScripts.scb'] : []),
    ].sort());
    const manifest = JSON.parse(args[2]);
    expect(manifest.additional.map(source => source.files[0])).toEqual(hasAdditional ? [{ name: 'z.big', blob: 0 }, { name: 'a.big', blob: 1 }] : []);
    expect(manifest.mod?.files[0].blob).toBe(hasMod ? (hasAdditional ? 2 : 0) : undefined);
    expect(args[3].map(bytes => new TextDecoder().decode(bytes))).toEqual([
      ...(hasAdditional ? ['first', 'second'] : []), ...(hasMod ? ['mod'] : []),
    ]);
  }
});

test('invalid inputs retain API errors and never reach retail assets', async () => {
  const worker = endpoint();
  const post = body => worker.fetch(new Request('https://example.test/api/checksum', { method: 'POST', body }), {});
  expect((await (await post(new FormData())).json()).error.code).toBe('missing_input');
  for (const game of ['', 'unknown', '__proto__', 'constructor']) {
    const form = new FormData();
    form.set('game', game);
    form.append('additional_file', new File(['ini'], 'patch.big'));
    form.set('additional_manifest', JSON.stringify([{ kind: 'big', files: [{ name: 'patch.big', index: 0 }] }]));
    expect((await (await post(form)).json()).error.code).toBe('game_required');
  }
  for (const manifest of [
    'not-json', '{}', '[]',
    JSON.stringify([{ kind: 'big', files: [{ name: 'patch.big', index: 5 }] }]),
    JSON.stringify([{ kind: 'folder', files: [{ name: '../GameData.ini', index: 0 }] }]),
    JSON.stringify([{ kind: 'big', files: [{ name: 'GameData.ini', index: 0 }] }]),
  ]) {
    const form = new FormData();
    form.append('additional_file', new File(['ini'], 'patch.big'));
    form.set('additional_manifest', manifest);
    const response = await post(form);
    expect(response.status).toBe(400);
    expect((await response.json()).error.code).toBe('invalid_manifest');
  }
  const tooLarge = await worker.fetch(new Request('https://example.test/api/checksum', {
    method: 'POST', headers: { 'content-length': String(64 * 1024 * 1024 + 1) },
  }), {});
  expect(tooLarge.status).toBe(413);
  expect((await tooLarge.json()).error.code).toBe('upload_too_large');
});

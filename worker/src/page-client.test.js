import { test, expect, beforeEach } from 'bun:test';
import { initializePage } from './page-client.js';
import { page } from './page.js';

import { Element } from '../test/dom-harness.js';
let elements, clipboard, request;
const $ = id => elements[id];
beforeEach(() => {
  elements = Object.fromEntries([...page.matchAll(/id="([^"]+)"/g)].map(match => [match[1], new Element()]));
  $('form').elements = { game: $('game') }; $('game').value = 'generalsmd';
  globalThis.document = { getElementById: $, createElement: () => new Element() };
  globalThis.window = { matchMedia: () => ({ matches: false }) };
  clipboard = [];
  Object.defineProperty(globalThis, 'navigator', { configurable: true, value: { clipboard: { writeText: async value => clipboard.push(value) } } });
  globalThis.fetch = async (url, init) => {
    request = { url, ...init };
    return { ok: true, json: async () => ({ game: 'generalsmd', version: '1.04', exe_crc: 'A7471E47' }) };
  };
  initializePage();
});

async function addExe() {
  $('exe-picker').files = [new File(['exe'], 'Game.dat')];
  await $('exe-picker').dispatch('change');
}

test('file and folder actions are separate; dropzones never intercept clicks or keys', async () => {
  for (const kind of ['additional', 'mod']) {
    await $(kind + '-folder-link').click();
    expect($(kind + '-folder-picker').clicks).toBe(1);
    expect($(kind + '-picker').clicks).toBe(0);
    expect($(kind + '-zone').listeners.click).toBeUndefined();
    expect($(kind + '-zone').listeners.keydown).toBeUndefined();
    // Enter/Space use the browser's native button activation, not a custom handler.
    for (const key of ['Enter', ' ']) {
      await $(kind + '-folder-link').dispatch('keydown', { key });
      await $(kind + '-folder-link').click();
    }
    expect($(kind + '-folder-picker').clicks).toBe(3);
    expect($(kind + '-picker').clicks).toBe(0);
    expect(page).toContain('id="' + kind + '-folder-link" class="picker-button" type="button"');
  }
});

test('empty inputs disable submission, EXE selection enables it, result has its own CRC card', async () => {
  expect($('submit').disabled).toBe(true);
  await addExe();
  expect($('submit').disabled).toBe(false);
  expect($('game-field').hidden).toBe(true);
  await $('form').dispatch('submit');
  expect(request.url).toBe('/api/checksum');
  expect(request.body.get('exe').name).toBe('Game.dat');
  expect($('results').dataset.state).toBe('success');
  expect($('status').textContent).toBe('');
  expect($('status').hidden).toBe(true);
  expect($('result-title').textContent).toBe('Zero Hour');
  expect($('fields').children[0].children.map(element => element.textContent)).toEqual(['Game version', '1.04']);
  expect($('exe_crc-value').textContent).toBe('A7471E47');
  expect($('ini_crc-card').hidden).toBe(true);
  expect($('results').focused).toBe(true);
  await $('copy-result').click();
  expect(JSON.parse(clipboard[0]).exe_crc).toBe('A7471E47');
  await $('exe_crc-copy').click();
  expect(clipboard[1]).toBe('A7471E47');
  expect($('copy-status').textContent).toBe('EXE CRC copied.');
  expect($('status').textContent).toBe('');
});

test('INI-only results promote game identity without an empty metadata row', async () => {
  globalThis.fetch = async () => ({ ok: true, json: async () => ({ game: 'generals', ini_crc: '81FB5632' }) });
  $('additional-picker').files = [new File(['big'], 'patch.big')];
  await $('additional-picker').dispatch('change');
  await $('form').dispatch('submit');
  expect($('result-title').textContent).toBe('Generals');
  expect($('ini_crc-value').textContent).toBe('81FB5632');
  expect($('exe_crc-card').hidden).toBe(true);
  expect($('fields').hidden).toBe(true);
  expect($('fields').children).toHaveLength(0);
});

test('combined results group version metadata before both checksum cards', async () => {
  globalThis.fetch = async () => ({ ok: true, json: async () => ({
    game: 'generalsmd', version: '1.04', generalsonline_version: 'test-build',
    exe_crc: 'A7471E47', ini_crc: '81FB5632',
  }) });
  await addExe(); await $('form').dispatch('submit');
  expect($('result-title').textContent).toBe('Zero Hour');
  expect($('fields').hidden).toBe(false);
  expect($('fields').children.map(field => field.children.map(element => element.textContent))).toEqual([
    ['Game version', '1.04'], ['Generals Online', 'test-build'],
  ]);
  expect($('exe_crc-card').hidden).toBe(false);
  expect($('ini_crc-card').hidden).toBe(false);
  expect(page.indexOf('id="fields"')).toBeLessThan(page.indexOf('id="result-values"'));
});

test('changed files flag prior results and cannot copy stale CRCs', async () => {
  await addExe(); await $('form').dispatch('submit');
  await $('exe-list').children[0].children[1].click();
  expect($('result-badge').textContent).toBe('Inputs changed');
  expect($('copy-result').hidden).toBe(true);
  expect($('results').classList.contains('stale')).toBe(true);
  await $('copy-result').click();
  expect($('exe_crc-copy').disabled).toBe(true);
  await $('exe_crc-copy').click();
  expect(clipboard).toHaveLength(0);
  expect($('submit').disabled).toBe(true);
});

test('in-flight uploads lock inputs and reject late input changes', async () => {
  let finish;
  globalThis.fetch = () => new Promise(resolve => { finish = resolve; });
  await addExe();
  const pending = $('form').dispatch('submit');
  expect($('inputs').disabled).toBe(true);
  expect($('results').dataset.state).toBe('loading');
  expect($('loading-result').hidden).toBe(false);
  expect($('empty-result').hidden).toBe(true);
  expect($('status').textContent).toBe('');
  $('exe-picker').files = [new File(['other'], 'other.exe')];
  await $('exe-picker').dispatch('change');
  expect($('exe-list').children[0].children[0].textContent).toBe('Game.dat');
  finish({ ok: true, json: async () => ({ game: 'generalsmd', exe_crc: 'A7471E47' }) });
  await pending;
  expect($('inputs').disabled).toBe(false);
  expect($('results').dataset.state).toBe('success');
  expect($('loading-result').hidden).toBe(true);
});

test('API failures are prominent and leave inputs available for correction', async () => {
  globalThis.fetch = async () => ({ ok: false, json: async () => ({ error: { message: 'Invalid executable' } }) });
  await addExe(); await $('form').dispatch('submit');
  expect($('results').dataset.state).toBe('error');
  expect($('result-note').textContent).toBe('Invalid executable');
  expect($('result-values').hidden).toBe(true);
  for (const id of ['empty-result', 'loading-result', 'fields', 'copy-result', 'result-json']) {
    expect($(id).hidden).toBe(true);
  }
  expect($('status').textContent).toBe('');
  expect($('status').hidden).toBe(true);
  expect($('copy-status').textContent).toBe('');
  expect($('inputs').disabled).toBe(false);
});

test('upload validation stays local and clears before a request', async () => {
  $('exe-picker').files = [new File(['invalid'], 'readme.txt')];
  await $('exe-picker').dispatch('change');
  expect($('status').textContent).toBe('Choose one .exe or .dat executable.');
  expect($('status').hidden).toBe(false);
  await addExe();
  expect($('status').hidden).toBe(true);
  await $('form').dispatch('submit');
  expect($('status').textContent).toBe('');
});

test('failed recalculation hides previous checksum and JSON controls', async () => {
  await addExe(); await $('form').dispatch('submit');
  await $('copy-result').click();
  globalThis.fetch = async () => ({ ok: false, json: async () => ({ error: { message: 'Choose the game executable, not the launcher.' } }) });
  await $('form').dispatch('submit');
  expect($('result-note').textContent).toBe('Choose the game executable, not the launcher.');
  for (const id of ['empty-result', 'loading-result', 'result-values', 'fields', 'copy-result', 'result-json']) {
    expect($(id).hidden).toBe(true);
  }
  expect($('copy-status').textContent).toBe('');
  expect($('status').textContent).toBe('');
});

test('folder paths and mod/archive multipart manifests remain intact', async () => {
  const file = new File(['ini'], 'GameData.ini');
  Object.defineProperty(file, 'webkitRelativePath', { value: 'MyPatch/Data/INI/GameData.ini' });
  $('additional-folder-picker').files = [file];
  await $('additional-folder-picker').dispatch('change');
  $('mod-picker').files = [new File(['big'], 'mod.big')];
  await $('mod-picker').dispatch('change');
  await $('form').dispatch('submit');
  expect(JSON.parse(request.body.get('additional_manifest'))).toEqual([
    { kind: 'folder', name: 'MyPatch', files: [{ name: 'Data/INI/GameData.ini', index: 0 }] },
  ]);
  expect(JSON.parse(request.body.get('mod_manifest')).files).toEqual([{ name: 'mod.big', index: 0 }]);
  expect(request.body.get('additional_file').name).toBe('GameData.ini');
});

test('retail automatically loads on entry and game change, without a show button or submit', async () => {
  const requests = [];
  globalThis.fetch = async (url, init) => {
    request = {url, ...init};
    requests.push(request);
    return {ok: true, json: async () => url.endsWith('generalsmd')
      ? {game: 'generalsmd', version: '1.04', ini_crc: 'FEAAE3F3'}
      : {game: 'generals', version: '1.08', ini_crc: '98FA8CC7'}};
  };
  await $('mode-retail').click();
  expect(requests).toEqual([{url: '/api/retail?game=generalsmd', method: 'GET'}]);
  expect($('ini_crc-value').textContent).toBe('FEAAE3F3');
  expect($('submit').hidden).toBe(true);
  expect($('form-footer').hidden).toBe(true);
  expect($('results').focused).not.toBe(true);
  $('game').value = 'generals'; await $('game').dispatch('change');
  expect($('upload-fields').hidden).toBe(true);
  expect($('game-field').hidden).toBe(false);
  expect($('submit').disabled).toBe(false);
  expect($('submit').hidden).toBe(true);
  expect($('retail-title').textContent).toBe('Generals 1.08');
  // Implicit form submission and reselecting the current mode do not refetch.
  await $('form').dispatch('submit');
  await $('mode-retail').click();
  expect(requests).toHaveLength(2);
  expect(request).toEqual({url: '/api/retail?game=generals', method: 'GET'});
  expect($('result-title').textContent).toBe('Generals');
  expect($('ini_crc-value').textContent).toBe('98FA8CC7');
  expect($('exe_crc-card').hidden).toBe(true);
  expect($('result-note').textContent).toContain('Unmodified retail');
});

test('retail ignores saved uploads and switching back restores them without stale copying', async () => {
  await addExe();
  await $('mode-retail').click();
  expect($('game-field').hidden).toBe(false);
  expect(request.method).toBe('GET');
  expect(request.body).toBeUndefined();
  await $('mode-files').click();
  expect($('submit').hidden).toBe(false);
  expect($('form-footer').hidden).toBe(false);
  expect($('submit').textContent).toBe('Calculate checksums');
  expect($('upload-fields').hidden).toBe(false);
  expect($('exe-list').children[0].children[0].textContent).toBe('Game.dat');
  expect($('game-field').hidden).toBe(true);
  expect($('result-badge').textContent).toBe('Inputs changed');
  expect($('result-json').hidden).toBe(true);
  await $('copy-result').click(); expect(clipboard).toHaveLength(0);
  await $('form').dispatch('submit');
  expect(request.method).toBe('POST');
  expect(request.body.get('exe').name).toBe('Game.dat');
});

test('retail requests lock mode switching until completion', async () => {
  let finish;
  globalThis.fetch = () => new Promise(resolve => {finish = resolve;});
  const pending = $('mode-retail').click();
  expect($('submit').hidden).toBe(true);
  await $('mode-files').click();
  expect($('upload-fields').hidden).toBe(true);
  expect($('inputs').disabled).toBe(true);
  finish({ok: true, json: async () => ({game: 'generalsmd', version: '1.04', ini_crc: 'FEAAE3F3'})});
  await pending;
  expect($('result-note').textContent).toContain('Unmodified retail');
  expect($('inputs').disabled).toBe(false);
  await $('mode-files').click();
  expect($('upload-fields').hidden).toBe(false);
});

test('retail failures expose retry only until the automatic baseline succeeds', async () => {
  let calls = 0;
  globalThis.fetch = async () => {
    calls++;
    return calls === 1
      ? {ok: false, json: async () => ({error: {message: 'Snapshot unavailable'}})}
      : {ok: true, json: async () => ({game: 'generalsmd', version: '1.04', ini_crc: 'FEAAE3F3'})};
  };
  await $('mode-retail').click();
  expect($('results').dataset.state).toBe('error');
  expect($('result-note').textContent).toBe('Snapshot unavailable');
  expect($('submit').hidden).toBe(false);
  expect($('submit').textContent).toBe('Retry retail checksum');
  expect($('submit').disabled).toBe(false);
  expect($('inputs').disabled).toBe(false);
  await $('form').dispatch('submit');
  expect(calls).toBe(2);
  expect($('results').dataset.state).toBe('success');
  expect($('submit').hidden).toBe(true);
  expect($('ini_crc-value').textContent).toBe('FEAAE3F3');
});

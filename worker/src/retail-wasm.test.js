import { beforeAll, expect, test } from 'bun:test';
import '../generated/wasm_exec.js';
import { baselineArchive } from '../test/fixtures.js';

beforeAll(async () => {
  const go = new globalThis.Go();
  const { instance } = await WebAssembly.instantiate(
    await Bun.file(new URL('../generated/gencrc.wasm', import.meta.url)).arrayBuffer(),
    go.importObject,
  );
  void go.run(instance);
});

for (const [name, bytes, code] of [
  ['launcher', 'Launcher config file missing\0launcher.cfg', 'launcher_executable'],
  ['unrelated file', 'not a game', 'unsupported_executable'],
  ['unrecognized game build', 'generalsmd', 'unsupported_executable'],
]) {
  test(`WASM gives actionable feedback for ${name}`, () => {
    const exe = new TextEncoder().encode(bytes), empty = new Uint8Array();
    const inspected = JSON.parse(globalThis.genCRCInspectExecutable(exe));
    expect(inspected.error_code).toBe(code);
    expect(inspected.error).toContain('Game.dat');
    expect(inspected.error).not.toContain('Version::setVersion');
    expect(JSON.parse(globalThis.genCRCCalculate('', exe, '{}', [], '', empty, empty, empty))).toEqual(inspected);
  });
}

for (const [game, archive] of [['generals', 'INI.big'], ['generalsmd', 'INIZH.big']]) {
  test(`WASM calculates ${game} baseline without executable or overlays`, () => {
    const empty = new Uint8Array();
    const result = JSON.parse(globalThis.genCRCCalculate(
      game, empty, '{"additional":[],"mod":null}', [], archive,
      baselineArchive(), empty, empty,
    ));
    // One normalized byte A: accumulator 0x41, byte-swapped output 0x41000000.
    expect(result).toEqual({ game, ini_crc: '41000000' });
    expect(result.exe_crc).toBeUndefined();
  });
}

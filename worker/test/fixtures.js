// Synthetic BIG containing one normalized byte (A); no retail game data.
export function baselineArchive() {
  const path = new TextEncoder().encode('Data\\INI\\GameData.ini');
  const offset = 16 + 8 + path.length + 1;
  const data = new Uint8Array(offset + 2);
  data.set(new TextEncoder().encode('BIGF'));
  const header = new DataView(data.buffer);
  header.setUint32(4, data.length);
  header.setUint32(8, 1);
  header.setUint32(12, offset);
  header.setUint32(16, offset);
  header.setUint32(20, 2);
  data.set(path, 24);
  data.set([65, 10], offset);
  return data;
}

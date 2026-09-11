const icon = (kind) => `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${{
  exe: '<rect x="4" y="3" width="16" height="18" rx="2"/><path d="m9 8 4 4-4 4m6 0h2"/>',
  additional: '<path d="M3 7a2 2 0 0 1 2-2h5l2 3h7a2 2 0 0 1 2 2v9H3Z"/><path d="M12 11v6m-3-3h6"/>',
  mod: '<path d="m12 3 9 5-9 5-9-5 9-5Zm-9 9 9 5 9-5M3 16l9 5 9-5"/>',
}[kind]}</svg>`;

function uploadZone(kind, title, hint) {
  return `
    <section class="upload-section" aria-labelledby="${kind}-title">
      <div id="${kind}-zone" class="dropzone">
        <div class="upload-icon">${icon(kind)}</div>
        <div class="drop-copy"><h3 id="${kind}-title">${title}</h3><span>${hint}</span></div>
        <div class="zone-actions">
          <button id="${kind}-file-link" class="picker-button" type="button">Add file</button>
          ${kind === 'exe' ? '' : `<button id="${kind}-folder-link" class="picker-button" type="button">Add folder</button>`}
          <span class="drop-hint">or drop here</span>
        </div>
      </div>
      <ol id="${kind}-list" class="selected" aria-label="Selected ${title}"></ol>
    </section>`;
}

export const page = String.raw`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <meta name="theme-color" content="#0a141b">
  <title>Gen//CRC</title>
  <link rel="icon" type="image/png" href="/logo.png">
  <style>
    :root { color-scheme: dark; --bg: #0e1112; --surface: #121718; --line: #3a494d; --text: #edf2f3; --muted: #b2c0c6; --cyan: #29e0ed; --danger: #ffb0a8; --mono: ui-monospace, SFMono-Regular, Consolas, monospace; }
    * { box-sizing: border-box; }
    [hidden] { display: none !important; }
    body { margin: 0; min-height: 100vh; background: linear-gradient(160deg, #12292d 0, #10191c 320px, var(--bg) 740px); color: var(--text); font: 14px/1.5 var(--mono); }
    button, select { font: inherit; }
    button, summary, select { cursor: pointer; }
    a { color: var(--cyan); text-underline-offset: 4px; }
    button:focus-visible, select:focus-visible, a:focus-visible, summary:focus-visible { outline: 2px solid var(--cyan); outline-offset: 4px; }
    button:disabled { cursor: not-allowed; opacity: .65; }
    .submit:disabled { opacity: 1; background: #233c40; border-color: #46636a; color: #b8c8cd; }
    h1, h2, h3, p { margin: 0; }
    .shell { max-width: 1240px; margin: auto; padding: 0 20px; }
    .site-header { border-bottom: 1px solid var(--line); background: linear-gradient(105deg, #153b40, #101b1e 75%); }
    .header-inner { min-height: 64px; display: flex; align-items: center; justify-content: space-between; gap: 16px; }
    .brand > div { min-width: 0; }
    .brand { display: flex; align-items: center; gap: 10px; min-width: 0; }
    .mark { width: 36px; height: 36px; flex-shrink: 0; object-fit: contain; }
    .wordmark { font: 700 22px/1.1 var(--mono); letter-spacing: -.5px; }
    .wordmark .gen { color: var(--cyan); }
    .wordmark .slashes { color: #637c8c; }
    .byline { color: #b0c3cc; font: 12px/1.4 var(--mono); margin-top: 3px; overflow-wrap: anywhere; }
    .eyebrow { color: var(--cyan); font: 12px/1.5 var(--mono); text-transform: uppercase; letter-spacing: 2px; }
    h1 { font-size: 18px; line-height: 1.3; font-weight: 650; letter-spacing: -.4px; margin: 4px 0; }
    .workspace { margin-top: 16px; display: grid; grid-template-columns: minmax(0, 2fr) minmax(0, 3fr); gap: 20px; align-items: start; }
    .input-panel { background: linear-gradient(150deg, #192528, var(--surface) 65%); border: 1px solid var(--line); border-radius: 0; }
    .panel-header { padding: 12px 16px; border-bottom: 1px solid var(--line); display: flex; justify-content: space-between; align-items: center; gap: 12px; }
    h2 { font-size: 16px; font-weight: 650; }
    .limit { color: var(--muted); font: 12px var(--mono); white-space: nowrap; }
    fieldset { border: 0; margin: 0; padding: 16px 16px 0; min-width: 0; }
    .game-field { display: grid; grid-template-columns: 1fr; gap: 8px; margin-bottom: 16px; }
    .game-field label { font-size: 16px; font-weight: 600; }
    .game-field small { display: block; color: var(--muted); font-size: 13px; font-weight: 400; margin-top: 0; }
    select { min-width: 160px; padding: 10px 30px 10px 12px; color: var(--text); background: #0a161e; border: 1px solid #405663; border-radius: 0; }
    .auto-game { margin-bottom: 16px; color: var(--muted); font-size: 13px; }
    .upload-section { padding-bottom: 12px; }
    .section-heading { margin-bottom: 8px; display: flex; align-items: center; justify-content: space-between; gap: 16px; }
    h3 { display: flex; align-items: center; gap: 8px; font-size: 16px; font-weight: 600; }
    .mode-switch { display: grid; grid-template-columns: 1fr 1fr; margin-bottom: 16px; border: 1px solid #405663; background: #0a1519; padding: 3px; gap: 3px; }
    .mode-switch button { border: 1px solid transparent; background: transparent; color: var(--muted); padding: 9px 6px; font-size: 14px; }
    .mode-switch button[aria-pressed="true"] { border-color: #329ca4; background: #173d42; color: #eaffff; font-weight: 600; }
    .mode-switch button:hover { color: white; background: #1b3035; }
    .retail-info { border-top: 1px solid var(--line); padding: 20px 0; }
    .retail-info h3 { margin: 8px 0; font-size: 22px; }
    .retail-info p { color: var(--muted); font-size: 14px; }
    .retail-info .retail-caveat { margin-top: 16px; font-size: 13px; }
    .step { color: #b0c3cc; font: 12px var(--mono); border: 1px solid #354a56; border-radius: 0; padding: 3px 5px; }
    .optional { font-size: 12px; color: #b0c3cc; }
    .section-description { color: var(--muted); font-size: 13px; margin: 6px 0 8px; }
    code { font-family: var(--mono); font-size: .88em; color: #a3e4e5; }
    .dropzone { display: grid; grid-template-columns: 28px minmax(0, 1fr); gap: 10px 12px; align-items: start; padding: 12px; border: 1px dashed #62828c; border-radius: 0; background: linear-gradient(125deg, #0d2025, #0a1519); transition: background .15s, border-color .15s; }
    .dropzone.drag { background: #123b40; border-color: var(--cyan); outline: 2px solid #43d4dc44; }
    .dropzone.has-files { border-color: #38747a; }
    .upload-icon { color: var(--cyan); width: 26px; height: 26px; }
    .upload-icon svg { width: 100%; height: 100%; }
    .drop-copy strong { font-size: 14px; font-weight: 500; display: block; }
    .drop-copy span { display: block; color: #b0c3cc; font-size: 13px; margin-top: 4px; }
    .zone-actions { grid-column: 2; display: flex; flex-wrap: wrap; align-items: center; gap: 8px; }
    .drop-hint { color: var(--muted); font-size: 13px; }
    .picker-button { border: 1px solid #3a515e; background: #172831; color: #d5e7eb; padding: 7px 12px; font-size: 14px; font-weight: 500; border-radius: 0; }
    .picker-button:hover { border-color: var(--cyan); color: white; background: #203c46; }
    .selected { list-style: none; margin: 8px 0 0; padding: 0; display: grid; gap: 5px; }
    .selected:empty { display: none; }
    .selected li { display: flex; align-items: center; gap: 12px; justify-content: space-between; padding: 5px 10px; background: #172c34; border-radius: 0; font-size: 13px; }
    .selected li span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
    .remove { border: 0; padding: 0 5px; background: none; color: #b6cbd1; font-size: 21px; line-height: 1.2; }
    .remove:hover { color: var(--danger); }
    .form-footer { border-top: 1px solid var(--line); padding: 12px 16px; }
    .submit { border: 1px solid var(--cyan); background: var(--cyan); color: #052026; padding: 12px 18px; width: 100%; font-size: 14px; font-weight: 700; border-radius: 0; }
    .submit:hover:not(:disabled) { background: #80e8eb; }
    .status { color: var(--muted); font-size: 13px; margin-top: 10px; }
    .status:empty { display: none; }
    .status.error { color: var(--danger); }
    .results { position: sticky; top: 16px; border: 1px solid var(--line); background: linear-gradient(135deg, #1a373b, #191f22 70%); min-height: 0; scroll-margin-top: 16px; }
    .results:focus { outline: none; }
    .result-kicker { display: flex; justify-content: space-between; align-items: center; gap: 10px; padding: 12px 16px; border-bottom: 1px solid var(--line); }
    .result-kicker .eyebrow { font-size: 16px; text-transform: none; letter-spacing: 0; font-weight: 650; }
    .result-body { padding: 20px 24px 24px; }
    .badge { color: #b8d6dc; font-size: 13px; }
    #result-title { font-size: 28px; line-height: 1.25; letter-spacing: -.6px; }
    #result-note { color: #bed2d9; font-size: 14px; margin-top: 9px; }
    .empty-result { padding: 37px 0 9px; }
    .empty-result span { display: block; color: #42616c; font: 31px var(--mono); letter-spacing: 3px; }
    .empty-result p { color: #b7ccd3; font-size: 13px; margin-top: 15px; max-width: 265px; }
    .result-values { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 230px), 1fr)); gap: 16px; margin: 24px 0; }
    .crc-card { padding: 16px; border: 1px solid #3d6269; background: linear-gradient(135deg, #153238, #111e22); }
    .crc-heading { display: flex; justify-content: space-between; align-items: center; }
    .crc-label { font: 13px var(--mono); color: #a0bec8; text-transform: uppercase; letter-spacing: 1.4px; }
    .crc-copy { color: #b1dfe0; background: transparent; border: 0; padding: 4px 0 4px 8px; font-size: 13px; }
    .crc-copy:hover { color: white; }
    .crc-value { font: 500 clamp(32px, 3.2vw, 40px)/1.4 var(--mono); color: #79ecdf; letter-spacing: 1px; overflow-wrap: anywhere; user-select: all; }
    .fields { display: flex; flex-wrap: wrap; gap: 12px 32px; margin: 20px 0 0; }
    .fields > div { display: grid; gap: 3px; }
    .fields dt { color: #bed2d9; font-size: 13px; }
    .fields dd { margin: 0; font-size: 17px; font-weight: 600; overflow-wrap: anywhere; }
    .copy-result { width: 100%; padding: 9px; border: 1px solid #52818a; border-radius: 0; background: transparent; color: #d6eeef; font-size: 13px; }
    .copy-result:hover { background: #24434b; }
    .result-json { margin-top: 15px; font-size: 13px; color: #bed2d9; }
    .results[data-state="error"] { border-color: #79504e; }
    .results[data-state="error"] .result-kicker { border-bottom-color: #79504e; }
    .results[data-state="error"] .badge { color: var(--danger); }
    .results[data-state="error"] #result-note { color: var(--danger); }
    .loading-result { height: 3px; margin-top: 24px; background: var(--cyan); animation: pulse 1.2s infinite alternate; }
    .results.stale .crc-value { color: #94afb4; }
    @keyframes pulse { to { opacity: .25; } }
    .api { margin: 20px 0; border: 1px solid var(--line); background: #121718; }
    .api > summary { padding: 12px 16px; font-size: 13px; font-weight: 600; }
    .api > summary span { float: right; color: #b0c3cc; font: 12px/22px var(--mono); }
    .api-body { padding: 0 25px 25px; color: var(--muted); font-size: 13px; }
    .api-body h3 { color: var(--text); margin: 25px 0 10px; }
    .api-body p { margin: 9px 0; }
    .api-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 25px; }
    pre { background: #08141b; border: 1px solid #29424e; padding: 14px; border-radius: 0; overflow: auto; font: 13px/1.75 var(--mono); color: #b7d4da; white-space: pre-wrap; overflow-wrap: anywhere; }
    .table-scroll { overflow-x: auto; }
    table { width: 100%; border-collapse: collapse; text-align: left; font-size: 13px; }
    th, td { border-bottom: 1px solid var(--line); padding: 10px 12px 10px 0; vertical-align: top; }
    th { color: #d0e4e9; font-weight: 500; }
    @media (max-width: 850px) {
      .shell { padding: 0 22px; max-width: 660px; }
      .workspace { grid-template-columns: minmax(0, 1fr); gap: 20px; }
      .results { position: static; }
      .crc-value { font-size: 36px; }
      .api-grid { grid-template-columns: 1fr; gap: 0; }
    }
    @media (max-width: 430px) {
      .shell { padding: 0 15px; }
      .header-inner { min-height: 64px; flex-wrap: wrap; gap: 8px; padding-top: 10px; padding-bottom: 10px; }
      .mark { width: 36px; height: 36px; }
      .wordmark { font-size: 22px; }
      .byline { font-size: 12px; }
      .panel-header, .form-footer { padding: 12px 14px; }
      fieldset { padding: 14px 14px 0; }
      .game-field { gap: 12px; }
      select { min-width: 126px; }
      .dropzone { padding: 14px; gap: 8px; grid-template-columns: 29px minmax(0, 1fr); }
      .upload-icon { width: 25px; height: 25px; }
      .result-body { padding: 18px; }
      .api > summary, .api-body { padding-left: 18px; padding-right: 18px; }
      .api > summary span { display: none; }
    }
    @media (prefers-reduced-motion: reduce) { *, *::before, *::after { animation: none !important; transition: none !important; scroll-behavior: auto !important; } }
  </style>
</head>
<body>
  <header class="site-header">
    <div class="shell header-inner">
      <div class="brand"><img class="mark" src="/logo.png" alt="Community Outpost"><div>
        <div class="wordmark" aria-label="Gen//CRC"><span class="gen">Gen</span><span class="slashes">//</span>CRC</div>
        <p class="byline">By ArcticDolphin @ Community Outpost</p>
      </div></div>
    </div>
  </header>
  <main class="shell">
    <div class="workspace">
      <form id="form" class="input-panel">
        <div class="panel-header"><h2>Checksum setup</h2><span id="upload-limit" class="limit">64 MiB max</span></div>
        <fieldset id="inputs" aria-label="Checksum inputs">
          <div class="mode-switch" role="group" aria-label="Checksum source">
            <button id="mode-files" type="button" aria-pressed="true">Your files</button>
            <button id="mode-retail" type="button" aria-pressed="false">Retail baseline</button>
          </div>
          <div id="game-field" class="game-field"><label for="game">Game</label><select id="game" name="game" aria-describedby="game-help"><option value="generalsmd">Zero Hour</option><option value="generals">Generals</option></select><small id="game-help">Or add an executable to detect the game.</small></div>
          <p id="game-auto" class="auto-game" hidden>Game will be detected from your executable.</p>
          <div id="upload-fields">
          ${uploadZone('exe', 'Game Executable', 'Game.dat or a game .exe · version &amp; EXE CRC')}
          ${uploadZone('additional', 'Additional Files', 'Like adding BIGs or loose files to your game folder.')}
          ${uploadZone('mod', 'Mod Files', '<code>-mod</code>: one BIG or a folder of BIGs. No loose files.')}
          </div>
          <div id="retail-info" class="retail-info" hidden><span class="eyebrow">Unmodified retail</span><h3 id="retail-title">Zero Hour 1.04</h3><p>The original INI checksum, calculated from our bundled retail snapshot. No files to upload.</p><p class="retail-caveat">EXE CRC requires your executable and is not included.</p></div>
          <input id="exe-picker" type="file" accept=".exe,.dat" hidden>
          <input id="additional-picker" type="file" accept=".big" multiple hidden>
          <input id="additional-folder-picker" type="file" webkitdirectory multiple hidden>
          <input id="mod-picker" type="file" accept=".big" hidden>
          <input id="mod-folder-picker" type="file" webkitdirectory multiple hidden>
        </fieldset>
        <div id="form-footer" class="form-footer"><button id="submit" class="submit" type="submit" disabled>Calculate checksums</button><p id="status" class="status" role="status" aria-live="polite" hidden></p></div>
      </form>
      <section id="results" class="results" tabindex="-1" aria-labelledby="result-title" data-state="empty">
        <div class="result-kicker"><span class="eyebrow">Result</span><span id="result-badge" class="badge">Awaiting files</span></div>
        <div class="result-body">
        <h2 id="result-title">Ready to calculate</h2>
        <p id="result-note" role="status" aria-live="polite">Select files and calculate to see EXE and INI CRCs.</p>
        <div id="loading-result" class="loading-result" aria-hidden="true" hidden></div>
        <dl id="fields" class="fields" hidden></dl>
        <div id="empty-result" class="empty-result" aria-hidden="true"><span>— — — —</span><p id="empty-help">EXE CRC from your executable.<br>INI CRC from Additional or Mod Files.</p></div>
        <div id="result-values" class="result-values" hidden>
          <div id="exe_crc-card" class="crc-card"><div class="crc-heading"><p class="crc-label">EXE CRC</p><button id="exe_crc-copy" type="button" class="crc-copy" aria-label="Copy EXE CRC">Copy</button></div><p id="exe_crc-value" class="crc-value"></p></div>
          <div id="ini_crc-card" class="crc-card"><div class="crc-heading"><p class="crc-label">INI CRC</p><button id="ini_crc-copy" type="button" class="crc-copy" aria-label="Copy INI CRC">Copy</button></div><p id="ini_crc-value" class="crc-value"></p></div>
        </div>
        <button id="copy-result" class="copy-result" type="button" hidden>Copy result JSON</button>
        <details id="result-json" class="result-json" hidden><summary>View raw JSON</summary><pre id="raw"></pre></details>
        <p id="copy-status" class="status" role="status" aria-live="polite"></p>
        </div>
      </section>
    </div>
    <details id="api" class="api"><summary>API reference <span>Endpoints &amp; examples</span></summary><div class="api-body">
      <p>Use <code>POST /api/checksum</code> with <code>multipart/form-data</code>. Send an executable, Additional Files, Mod Files, or a combination. <a href="/api">GET /api</a> returns the machine-readable contract.</p>
      <div class="api-grid"><section><h3>Executable only</h3><pre>curl -X POST https://YOUR_HOST/api/checksum \
  -F 'exe=@Game.dat'</pre><p>The executable determines the game; <code>game</code> is not needed.</p></section><section><h3>One community patch</h3><pre>curl -X POST https://YOUR_HOST/api/checksum \
  -F 'game=generalsmd' \
  -F 'additional_manifest=[{"kind":"big","files":[{"name":"patch.big","index":0}]}]' \
  -F 'additional_file=@patch.big'</pre></section></div>
      <h3>Retail baseline - no upload</h3><pre>curl 'https://YOUR_HOST/api/retail?game=generalsmd'</pre><p>Use <code>generalsmd</code> for Zero Hour 1.04 or <code>generals</code> for Generals 1.08. Returns <code>game</code>, <code>version</code>, and <code>ini_crc</code> from the bundled retail snapshot. No EXE CRC: executable bytes vary between distributions.</p>
      <h3>Upload request fields</h3>
      <div class="table-scroll"><table><thead><tr><th>Field</th><th>Value</th></tr></thead><tbody>
        <tr><td><code>exe</code></td><td>Optional executable file. Adds EXE CRC and version metadata.</td></tr>
        <tr><td><code>game</code></td><td><code>generals</code> or <code>generalsmd</code> (Zero Hour). Required without an executable; ignored when one is supplied.</td></tr>
        <tr><td><code>additional_manifest</code></td><td>JSON array of BIG or folder sources, describing the <code>additional_file</code> uploads.</td></tr>
        <tr><td><code>additional_file</code></td><td>Repeat for each file. Manifest indices refer to these parts, starting at zero.</td></tr>
        <tr><td><code>mod_manifest</code></td><td>One BIG or folder source object, describing the <code>mod_file</code> uploads.</td></tr>
        <tr><td><code>mod_file</code></td><td>Repeat for each mod archive. Indices restart at zero for mod files.</td></tr>
      </tbody></table></div>
      <h3>Folders and multiple files</h3><p>A BIG source has <code>kind: "big"</code> and exactly one file. A folder has <code>kind: "folder"</code>, an optional <code>name</code>, and relative file paths. Do not include the folder's outer name in those paths. Mod folders load BIG archives only.</p>
      <pre>[
  {"kind":"folder","name":"MyPatch","files":[
    {"name":"Data/INI/GameData.ini","index":0},
    {"name":"patch.big","index":1}
  ]}
]</pre><p>Append the corresponding files as <code>additional_file</code> parts in index order. For a mod, send one object (not an array) as <code>mod_manifest</code> and use <code>mod_file</code> parts. Paths must be relative and cannot contain <code>..</code>.</p>
      <div class="api-grid"><section><h3>Success · 200</h3><pre>{
  "game": "generalsmd",
  "version": "1.04",
  "exe_crc": "A7471E47",
  "ini_crc": "FEAAE3F3"
}</pre><p>Example combined result. CRCs are eight-digit uppercase hex. <code>exe_crc</code> and <code>version</code> require an executable; <code>ini_crc</code> requires Additional or Mod Files. <code>generalsonline_version</code> is included when recognized.</p></section><section><h3>Errors</h3><pre>{"error":{
  "code":"missing_input",
  "message":"Provide an executable, Additional Files, or Mod Files"
}}</pre><p><code>400</code> missing input or invalid manifest; <code>404</code> unknown endpoint; <code>413</code> upload exceeds 64 MiB; <code>422</code> invalid game, unsupported executable, or checksum failure.</p><p><code>launcher_executable</code> identifies a launcher: upload <code>Game.dat</code> instead. <code>unsupported_executable</code> means the game or version could not be recognized.</p><p>For <code>POST /api/checksum</code>, at least one non-empty upload is required. Combined uploads are limited to 64 MiB. JSON responses allow cross-origin requests and are not cached. Both checksum endpoints support <code>OPTIONS</code> browser preflight.</p></section></div>
    </div></details>
  </main>
  <script type="module">import { initializePage } from "/page-client.js"; initializePage();</script>
</body>
</html>`;

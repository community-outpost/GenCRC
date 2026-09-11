// Served unchanged as a browser module by the Worker (Wrangler Text rule).
export function initializePage() {
  const $ = id => document.getElementById(id);
  const form = $('form');
  let exeFile = null, additionalSources = [], modSource = null;
  let currentResult = null, busy = false, mode = 'files', inputRevision = 0;
  const isBig = file => /\.big$/i.test(file.name);
  const isExe = file => /\.(exe|dat)$/i.test(file.name);

  function announce(message, error = false) {
    $('status').textContent = message;
    $('status').hidden = !message;
    $('status').classList.toggle('error', error);
  }

  function invalidate() {
    inputRevision++;
    announce('');
    $('copy-status').textContent = '';
    if (currentResult) {
      $('result-badge').textContent = 'Inputs changed';
      $('result-note').textContent = 'Previous result. Run the current setup to refresh your checksums.';
      $('results').classList.add('stale');
      $('copy-result').hidden = true;
      $('result-json').hidden = true;
      for (const key of ['exe_crc', 'ini_crc']) $(key + '-copy').disabled = true;
      currentResult = null;
    }
  }

  function row(label, remove) {
    const li = document.createElement('li');
    const name = document.createElement('span');
    name.textContent = label;
    name.title = label;
    const button = document.createElement('button');
    button.type = 'button';
    button.className = 'remove';
    button.textContent = '×';
    button.setAttribute('aria-label', 'Remove ' + label);
    button.onclick = () => { if (!busy) { remove(); renderSources(); invalidate(); } };
    li.append(name, button);
    return li;
  }

  const sourceLabel = source => source.kind === 'big'
    ? source.files[0].file.name
    : source.name + ' / ' + source.files.length + ' files';

  function renderSources() {
    $('exe-list').replaceChildren(...(exeFile ? [row(exeFile.name, () => { exeFile = null; })] : []));
    $('additional-list').replaceChildren(...additionalSources.map((source, index) =>
      row(sourceLabel(source), () => { additionalSources.splice(index, 1); })));
    $('mod-list').replaceChildren(...(modSource ? [row(sourceLabel(modSource), () => { modSource = null; })] : []));
    const retail = mode === 'retail';
    $('game-field').hidden = !retail && !!exeFile;
    $('game-auto').hidden = retail || !exeFile;
    $('upload-fields').hidden = retail;
    $('upload-limit').hidden = retail;
    $('retail-info').hidden = !retail;
    $('empty-help').textContent = retail ? 'Generals 1.08 or Zero Hour 1.04. No files required.' : 'EXE CRC from your executable. INI CRC from Additional or Mod Files.';
    $('game-help').textContent = retail ? 'Choose a retail version.' : 'Or add an executable to detect the game.';
    $('retail-title').textContent = form.elements.game.value === 'generals' ? 'Generals 1.08' : 'Zero Hour 1.04';
    $('mode-files').setAttribute('aria-pressed', String(!retail));
    $('mode-retail').setAttribute('aria-pressed', String(retail));
    $('submit').hidden = retail && (busy || $('results').dataset.state !== 'error');
    $('form-footer').hidden = retail && $('submit').hidden;
    $('submit').textContent = busy ? 'Calculating...' : retail ? 'Retry retail checksum' : 'Calculate checksums';
    $('submit').disabled = busy || !(retail || exeFile || additionalSources.length || modSource);
    for (const kind of ['exe', 'additional', 'mod']) {
      $(kind + '-zone').classList.toggle('has-files', !!$(kind + '-list').children.length);
    }
  }

  function accept(source, kind) {
    if (kind === 'exe') {
      if (source.kind !== 'big' || !isExe(source.files[0].file)) throw new Error('Choose one .exe or .dat executable.');
      exeFile = source.files[0].file;
    } else {
      if (source.kind === 'big' && !isBig(source.files[0].file)) throw new Error('Choose a .big archive, or add a folder.');
      if (kind === 'mod') {
        if (modSource) throw new Error('Remove the current Mod Files source before adding another.');
        source.files = source.files.filter(item => isBig(item.file));
        if (!source.files.length) throw new Error('This mod folder contains no .big archives. Loose mod files are not loaded by the game.');
        modSource = source;
      } else {
        if (!source.files.length) throw new Error('This folder is empty.');
        additionalSources.push(source);
      }
    }
    renderSources();
    invalidate();
  }

  function acceptSources(sources, kind) {
    if (busy || mode !== 'files') return;
    try {
      if (kind !== 'additional' && sources.length !== 1) throw new Error('Choose exactly one ' + (kind === 'exe' ? 'executable.' : 'BIG or folder for Mod Files.'));
      for (const source of sources) accept(source, kind);
      announce('');
    } catch (error) { announce(error.message, true); }
  }

  function folderSource(files) {
    const root = files[0].webkitRelativePath.split('/')[0];
    return { kind: 'folder', name: root, files: files.map(file => ({
      name: file.webkitRelativePath.split('/').slice(1).join('/'), file,
    })) };
  }

  async function readEntry(entry, path = '') {
    if (entry.isFile) return new Promise((resolve, reject) => entry.file(file => resolve([{ name: path + file.name, file }]), reject));
    if (!entry.isDirectory) return [];
    const reader = entry.createReader(), children = [];
    for (;;) {
      const batch = await new Promise((resolve, reject) => reader.readEntries(resolve, reject));
      if (!batch.length) break;
      children.push(...batch);
    }
    return (await Promise.all(children.map(child => readEntry(child, path + entry.name + '/')))).flat();
  }

  for (const kind of ['exe', 'additional', 'mod']) {
    const zone = $(kind + '-zone');
    const picker = $(kind + '-picker');
    // Only the explicit button opens a picker. Drop containers have no click handler.
    $(kind + '-file-link').onclick = () => picker.click();
    picker.onchange = () => {
      const files = [...picker.files];
      if (files.length) acceptSources(files.map(file => ({ kind: 'big', files: [{ name: file.name, file }] })), kind);
      picker.value = '';
    };
    if (kind !== 'exe') {
      const folderPicker = $(kind + '-folder-picker');
      $(kind + '-folder-link').onclick = () => folderPicker.click();
      folderPicker.onchange = () => {
        if (folderPicker.files.length) acceptSources([folderSource([...folderPicker.files])], kind);
        folderPicker.value = '';
      };
    }
    let dragDepth = 0;
    zone.addEventListener('dragenter', event => { event.preventDefault(); dragDepth++; zone.classList.add('drag'); });
    zone.addEventListener('dragover', event => { event.preventDefault(); event.dataTransfer.dropEffect = 'copy'; });
    zone.addEventListener('dragleave', () => { if (--dragDepth <= 0) zone.classList.remove('drag'); });
    zone.addEventListener('drop', async event => {
      event.preventDefault(); dragDepth = 0; zone.classList.remove('drag');
      if (busy || mode !== 'files') return;
      const revision = inputRevision;
      // Capture entries before awaiting: the browser clears the drag data store.
      const entries = [...event.dataTransfer.items].map(item => item.webkitGetAsEntry?.()).filter(Boolean);
      const fallback = [...event.dataTransfer.files];
      try {
        const sources = entries.length ? await Promise.all(entries.map(async entry => {
          const files = await readEntry(entry);
          return entry.isDirectory
            ? { kind: 'folder', name: entry.name, files: files.map(item => ({ ...item, name: item.name.split('/').slice(1).join('/') })) }
            : { kind: 'big', files };
        })) : fallback.map(file => ({ kind: 'big', files: [{ name: file.name, file }] }));
        if (revision === inputRevision) acceptSources(sources, kind);
      } catch (error) { announce(error.message, true); }
    });
  }

  function resultState(state, title, note) {
    $('results').dataset.state = state;
    $('result-title').textContent = title;
    $('result-note').textContent = note;
    $('result-badge').textContent = ({ empty: 'Awaiting files', loading: 'Working', error: 'Needs attention', success: 'Complete' })[state];
    $('empty-result').hidden = state !== 'empty';
    $('loading-result').hidden = state !== 'loading';
    $('result-values').hidden = state !== 'success';
    $('copy-result').hidden = state !== 'success';
    $('result-json').hidden = state !== 'success';
    $('fields').hidden = true;
    $('copy-status').textContent = '';
    announce('');
    $('results').classList.remove('stale');
  }

  function showResult(body) {
    currentResult = body;
    const gameName = body.game === 'generalsmd' ? 'Zero Hour' : body.game === 'generals' ? 'Generals' : 'Your checksums';
    resultState('success', gameName, mode === 'retail' ? 'Unmodified retail INI baseline. No executable checksum included.' : body.ini_crc ? 'Checksums for your files and the retail snapshot.' : 'Executable identified. Your EXE checksum is ready.');
    for (const key of ['exe_crc', 'ini_crc']) {
      $(key + '-card').hidden = !body[key];
      $(key + '-value').textContent = body[key] || '';
      $(key + '-copy').disabled = false;
    }
    const fields = $('fields');
    fields.replaceChildren();
    const labels = { version: 'Game version', generalsonline_version: 'Generals Online' };
    for (const [key, label] of Object.entries(labels)) {
      if (!body[key]) continue;
      const dt = document.createElement('dt'), dd = document.createElement('dd');
      dt.textContent = label;
      dd.textContent = body[key];
      const field = document.createElement('div');
      field.append(dt, dd);
      fields.append(field);
    }
    fields.hidden = fields.children.length === 0;
    $('raw').textContent = JSON.stringify(body, null, 2);
  }

  async function copy(value, label) {
    if (!currentResult) return;
    try {
      await navigator.clipboard.writeText(value);
      $('copy-status').textContent = label + ' copied.';
    } catch { $('copy-status').textContent = 'Could not access clipboard. Select and copy the raw JSON below.'; $('result-json').open = true; }
  }
  $('copy-result').onclick = () => copy(JSON.stringify(currentResult, null, 2), 'Result JSON');
  for (const key of ['exe_crc', 'ini_crc']) {
    $(key + '-copy').onclick = () => copy(currentResult?.[key] || '', key === 'exe_crc' ? 'EXE CRC' : 'INI CRC');
  };
  for (const next of ['files', 'retail']) {
    $('mode-' + next).onclick = () => {
      if (busy || mode === next) return;
      mode = next;
      invalidate();
      renderSources();
      if (!currentResult && $('results').dataset.state !== 'success') {
        resultState('empty', next === 'retail' ? 'Retail baseline' : 'Ready to calculate',
          next === 'retail' ? 'Choose a game to view its original retail INI checksum.' : 'Select files and calculate to see EXE and INI CRCs.');
        $('result-badge').textContent = next === 'retail' ? 'Ready' : 'Awaiting files';
      }
      announce('');
      if (next === 'retail') return calculate();
    };
  }
  form.elements.game.onchange = () => {
    if (busy) return;
    invalidate();
    renderSources();
    if (mode === 'retail') return calculate();
  };
  form.addEventListener('submit', event => {
    event.preventDefault();
    // Retail loads on selection; Enter must not repeat a completed request.
    if (mode === 'retail' && $('results').dataset.state !== 'error') return;
    return calculate();
  });

  async function calculate() {
    if (busy || !(mode === 'retail' || exeFile || additionalSources.length || modSource)) return;
    busy = true;
    currentResult = null;
    const retail = mode === 'retail';
    let data;
    if (!retail) {
      data = new FormData();
      data.set('game', form.elements.game.value);
      if (exeFile) data.set('exe', exeFile);
      const additional = [], additionalFiles = [];
      for (const source of additionalSources) {
        const files = source.files.map(item => ({ name: item.name, index: additionalFiles.push(item.file) - 1 }));
        additional.push({ kind: source.kind, name: source.name || '', files });
      }
      additionalFiles.forEach(file => data.append('additional_file', file));
      data.set('additional_manifest', JSON.stringify(additional));
      if (modSource) {
        modSource.files.forEach(item => data.append('mod_file', item.file));
        data.set('mod_manifest', JSON.stringify({ kind: modSource.kind, name: modSource.name || '',
          files: modSource.files.map((item, index) => ({ name: item.name, index })) }));
      }
    }
    $('inputs').disabled = true;
    resultState('loading', retail ? 'Reading retail snapshot' : 'Calculating checksums', retail ? 'Calculating the unmodified retail INI baseline.' : 'Reading your files. Larger uploads may take a moment.');
    renderSources();
    try {
      const response = await fetch(retail ? '/api/retail?game=' + encodeURIComponent(form.elements.game.value) : '/api/checksum', retail ? { method: 'GET' } : { method: 'POST', body: data });
      const body = await response.json();
      if (!response.ok) throw new Error(body.error?.message || 'Request failed. Please try again.');
      showResult(body);
    } catch (error) {
      currentResult = null;
      resultState('error', 'Could not calculate', error.message);
    } finally {
      busy = false;
      $('inputs').disabled = false;
      renderSources();
      // Automatic selection changes should not move keyboard focus or scroll.
      if (!retail) {
        $('results').focus({ preventScroll: true });
        if (window.matchMedia('(max-width: 850px)').matches) $('results').scrollIntoView({ behavior: 'smooth', block: 'start' });
      }
    }
  }
  renderSources();
}

const $ = s => document.querySelector(s);

function dot(el, kind) { el.className = 'dot' + (kind ? ' ' + kind : ''); }

function render(s) {
  if (!s) return;
  $('#enabled').checked = !!s.enabled;
  if (document.activeElement !== $('#clientId')) $('#clientId').value = s.clientId || '';

  const hostText = {
    idle: '—',
    starting: '啟動中…',
    running: '執行中',
    stopped: '已停止',
    missing: '沒有執行'
  }[s.host] || s.host;
  $('#host').textContent = hostText;
  dot($('#hostDot'), s.host === 'running' ? 'ok' : s.host === 'missing' ? 'bad' : s.host === 'stopped' ? 'warn' : '');

  $('#discord').textContent = s.discord
    ? ('已連線' + (s.user ? '（' + s.user + '）' : ''))
    : (s.host === 'running' ? (s.error || '待命（播放時才連線）') : '未連線');
  dot($('#discordDot'), s.discord ? 'ok' : s.host === 'running' ? 'warn' : '');

  const p = s.playing;
  $('#song').textContent = p ? (p.title + (p.artist ? ' — ' + p.artist : '') + (p.paused ? '（暫停）' : '')) : '沒有開啟 Simple Player';
  dot($('#songDot'), p && !p.paused ? 'ok' : '');

  const hint = $('#hint');
  if (s.host === 'missing') {
    hint.hidden = false;
    hint.innerHTML = '偵測不到 <code>SimplePlayerPresence.exe</code>。請執行一次安裝程式（之後會隨 Windows 自動啟動），再按「進階設定 → 重新連線」。';
  } else {
    hint.hidden = true;
  }
}

chrome.runtime.sendMessage({ type: 'getStatus' }, render);
chrome.runtime.onMessage.addListener(msg => { if (msg?.type === 'status') render(msg.status); });

$('#enabled').onchange = e => chrome.runtime.sendMessage({ type: 'setEnabled', enabled: e.target.checked }, render);
$('#saveCid').onclick = () => chrome.runtime.sendMessage({ type: 'setClientId', clientId: $('#clientId').value }, render);
$('#retry').onclick = () => chrome.runtime.sendMessage({ type: 'retry' }, render);

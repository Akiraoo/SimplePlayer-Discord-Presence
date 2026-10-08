// Simple Player Discord Presence — background service worker.
// Pages report what is playing; this worker forwards it to SimplePlayerPresence.exe
// (running in the background on 127.0.0.1), which talks to the Discord desktop app.

const HELPER = 'http://127.0.0.1:47823';
const DEFAULT_CLIENT_ID = '1552923544420225046';

let settings = { enabled: true, clientId: DEFAULT_CLIENT_ID };
let status = { host: 'idle', discord: false, user: '', error: '' };
const tabs = new Map(); // tabId -> { state, at }
let lastSent = '';
let lastSentAt = 0;

const ready = chrome.storage.local.get(['enabled', 'clientId']).then(v => {
  if (typeof v.enabled === 'boolean') settings.enabled = v.enabled;
  if (v.clientId) settings.clientId = String(v.clientId);
});

function setStatus(patch) {
  status = { ...status, ...patch };
  chrome.runtime.sendMessage({ type: 'status', status: publicStatus() }).catch(() => {});
}

function publicStatus() {
  const cur = currentState();
  return { ...status, enabled: settings.enabled, clientId: settings.clientId,
    playing: cur ? { title: cur.title, artist: cur.artist, paused: cur.paused } : null };
}

async function helper(path, body) {
  try {
    const r = await fetch(HELPER + path, {
      method: body === undefined ? 'GET' : 'POST',
      headers: { 'X-SimplePlayer': '1', 'Content-Type': 'application/json' },
      body: body === undefined ? undefined : JSON.stringify(body)
    });
    if (!r.ok) throw new Error('HTTP ' + r.status);
    const s = await r.json();
    setStatus({ host: 'running', discord: !!s.connected, user: s.user || '', error: s.error || '' });
    return true;
  } catch (e) {
    setStatus({ host: 'missing', discord: false, user: '', error: '' });
    return false;
  }
}

/** The most recently updated tab that is playing; otherwise the most recent one. */
function currentState() {
  let best = null;
  for (const v of tabs.values()) {
    if (!v.state) continue;
    if (!best ||
        (!v.state.paused && best.state.paused) ||
        (v.state.paused === best.state.paused && v.at > best.at)) best = v;
  }
  return best ? best.state : null;
}

function isPublicHttps(url) {
  try {
    const u = new URL(url);
    if (u.protocol !== 'https:') return false;
    const h = u.hostname;
    return !(h === 'localhost' || /^(10|127)\./.test(h) || /^192\.168\./.test(h) ||
      /^172\.(1[6-9]|2\d|3[01])\./.test(h) || h.endsWith('.local') || /^\[?[0-9a-f:]+\]?$/i.test(h));
  } catch { return false; }
}

function clip(text, fallback) {
  let s = String(text || '').trim() || fallback;
  if (s.length < 2) s = s + ' ';
  return s.length > 128 ? s.slice(0, 127) + '…' : s;
}

function buildActivity(st) {
  const activity = {
    type: 2, // "Listening to"
    details: clip(st.title, 'Unknown'),
    state: clip(st.artist || st.album, 'Simple Player'),
    assets: {},
    instance: false
  };
  if (Number.isFinite(st.duration) && st.duration > 0 && Number.isFinite(st.position)) {
    const start = Date.now() - Math.max(0, st.position) * 1000;
    activity.timestamps = { start: Math.round(start), end: Math.round(start + st.duration * 1000) };
  }
  if (st.paused) {
    // Discord has no "paused" flag. Keep the status, mark it with ⏸, and keep re-anchoring the
    // timestamps to the paused position (see push) so the progress bar looks frozen.
    activity.state = clip((st.artist || st.album || 'Simple Player') + ' · ⏸', 'Simple Player');
  }
  if (st.cover && isPublicHttps(st.cover)) {
    activity.assets.large_image = st.cover;
    if (st.album) activity.assets.large_text = clip(st.album, 'Simple Player');
  }
  if (st.url && isPublicHttps(st.url)) {
    activity.buttons = [{ label: '在 Simple Player 收聽', url: st.url }];
  }
  if (!Object.keys(activity.assets).length) delete activity.assets;
  return activity;
}

async function push(force = false) {
  await ready;
  const st = settings.enabled ? currentState() : null;
  const activity = st ? buildActivity(st) : null;
  const start = activity?.timestamps?.start;
  const key = JSON.stringify(activity
    ? { ...activity, timestamps: undefined, id: st.id, s: start ? Math.round(start / 3000) : 0 }
    : null);
  // Re-send the same state at most every 15 s (the "still alive" heartbeat). While paused,
  // re-send every 5 s so the frozen progress bar does not drift.
  const every = st && st.paused ? 4500 : 15000;
  if (!force && key === lastSent && Date.now() - lastSentAt < every) return;
  if (!activity && lastSent === 'null' && !force) return;
  lastSent = key;
  lastSentAt = Date.now();
  if (!(await helper('/presence', { clientId: settings.clientId, activity }))) lastSent = '';
}

chrome.runtime.onConnect.addListener(port => {
  if (port.name !== 'simpleplayer-tab') return;
  const tabId = port.sender?.tab?.id ?? -1;
  port.onMessage.addListener(msg => {
    if (msg?.type !== 'state') return;
    tabs.set(tabId, { state: msg.state, at: Date.now() });
    push();
  });
  port.onDisconnect.addListener(() => {
    tabs.delete(tabId);
    push();
  });
});

chrome.runtime.onMessage.addListener((msg, _sender, reply) => {
  (async () => {
    await ready;
    if (msg?.type === 'getStatus') {
      await helper('/status');
      reply(publicStatus());
    } else if (msg?.type === 'setEnabled') {
      settings.enabled = !!msg.enabled;
      await chrome.storage.local.set({ enabled: settings.enabled });
      await push(true);
      reply(publicStatus());
    } else if (msg?.type === 'setClientId') {
      settings.clientId = String(msg.clientId || '').trim() || DEFAULT_CLIENT_ID;
      await chrome.storage.local.set({ clientId: settings.clientId });
      await push(true);
      reply(publicStatus());
    } else if (msg?.type === 'retry') {
      await push(true);
      await helper('/status');
      reply(publicStatus());
    }
  })();
  return true;
});

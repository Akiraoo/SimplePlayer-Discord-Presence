// Runs on every page but stays idle until the page is the desktop Simple Player web
// player, which announces itself with "simpleplayer:presence" events (see public/app.js).
(() => {
  let port = null;
  let last = null;
  let timer = 0;
  let timerMs = 0;
  let everPlayed = false;

  function send(msg) {
    try {
      if (!port) {
        port = chrome.runtime.connect({ name: 'simpleplayer-tab' });
        port.onDisconnect.addListener(() => { port = null; });
      }
      port.postMessage(msg);
    } catch (e) {
      // Extension was reloaded or removed; nothing to do on this page any more.
      port = null;
      clearInterval(timer);
      timer = 0;
      timerMs = 0;
    }
  }

  window.addEventListener('simpleplayer:presence', e => {
    let state = null;
    try { state = typeof e.detail === 'string' ? JSON.parse(e.detail) : null; } catch {}
    // A track restored on page load is paused but was never played: show nothing for it.
    if (state && !state.paused) everPlayed = true;
    if (state && state.paused && !everPlayed) state = null;
    last = state;
    send({ type: 'state', state });
    // Repeat the state so the helper knows the tab is still alive: every 20 s while playing,
    // every 5 s while paused (the paused progress bar is re-anchored each time).
    const wantMs = !state ? 0 : state.paused ? 5000 : 20000;
    if (wantMs !== timerMs) {
      clearInterval(timer);
      timer = 0;
      timerMs = wantMs;
      if (wantMs) {
        timer = setInterval(() => window.dispatchEvent(new Event('simpleplayer:presence-request')), wantMs);
      }
    }
  });

  window.addEventListener('DOMContentLoaded', () => {
    window.dispatchEvent(new Event('simpleplayer:presence-request'));
  });
})();

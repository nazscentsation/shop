/* =============================================
   NazScentsation — Frontend JavaScript
   ============================================= */

// ---- Floating particles ----
(function initParticles() {
  const container = document.getElementById('particles');
  if (!container) return;

  for (let i = 0; i < 45; i++) {
    const p = document.createElement('span');
    p.className = 'particle';
    const size  = (Math.random() * 3 + 1).toFixed(1) + 'px';
    const left  = (Math.random() * 100).toFixed(1) + '%';
    const dur   = (Math.random() * 14 + 10).toFixed(1) + 's';
    const delay = (Math.random() * 14).toFixed(1) + 's';
    p.style.cssText = `--size:${size};--dur:${dur};--delay:${delay};left:${left};bottom:-10px;`;
    container.appendChild(p);
  }
})();


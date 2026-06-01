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

// ---- Countdown timer ----
(function initCountdown() {
  // Set your launch date here (YYYY, MM-1, DD, HH, MM, SS)
  const launch = new Date(2026, 8, 1, 0, 0, 0); // 1 Sep 2026

  const pad = n => String(n).padStart(2, '0');

  const daysEl  = document.getElementById('cd-days');
  const hoursEl = document.getElementById('cd-hours');
  const minsEl  = document.getElementById('cd-mins');
  const secsEl  = document.getElementById('cd-secs');

  if (!daysEl) return;

  function tick() {
    const now  = Date.now();
    const diff = launch.getTime() - now;

    if (diff <= 0) {
      daysEl.textContent = hoursEl.textContent = minsEl.textContent = secsEl.textContent = '00';
      return;
    }

    const days  = Math.floor(diff / 86400000);
    const hours = Math.floor((diff % 86400000) / 3600000);
    const mins  = Math.floor((diff % 3600000)  / 60000);
    const secs  = Math.floor((diff % 60000)    / 1000);

    daysEl.textContent  = pad(days);
    hoursEl.textContent = pad(hours);
    minsEl.textContent  = pad(mins);
    secsEl.textContent  = pad(secs);
  }

  tick();
  setInterval(tick, 1000);
})();

// ---- Notify form ----
(function initNotifyForm() {
  const form     = document.getElementById('notifyForm');
  const input    = document.getElementById('notifyEmail');
  const feedback = document.getElementById('notifyFeedback');
  const btn      = form ? form.querySelector('button') : null;

  if (!form) return;

  function setFeedback(msg, type) {
    feedback.textContent = msg;
    feedback.className   = 'notify-feedback ' + type;
  }

  form.addEventListener('submit', async (e) => {
    e.preventDefault();

    const email = input.value.trim();
    if (!email || !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
      setFeedback('Please enter a valid email address.', 'error');
      return;
    }

    btn.textContent = '...';
    btn.disabled    = true;

    try {
      const res = await fetch('/api/notify', {
        method:  'POST',
        headers: { 'Content-Type': 'application/json' },
        body:    JSON.stringify({ email }),
      });

      if (res.ok) {
        setFeedback("You're on the list. We'll be in touch.", 'success');
        input.value  = '';
      } else {
        const data = await res.json().catch(() => ({}));
        setFeedback(data.error || 'Something went wrong. Please try again.', 'error');
        btn.disabled    = false;
        btn.textContent = 'Notify Me';
      }
    } catch {
      setFeedback('Could not connect. Please try again later.', 'error');
      btn.disabled    = false;
      btn.textContent = 'Notify Me';
    }
  });
})();

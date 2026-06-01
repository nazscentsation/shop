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

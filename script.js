/* NazScentsation — Coming Soon Scripts */

// ---- Floating particles ----
(function spawnParticles() {
  const container = document.getElementById('particles');
  const count = 40;

  for (let i = 0; i < count; i++) {
    const p = document.createElement('span');
    p.className = 'particle';

    const size  = (Math.random() * 3 + 1).toFixed(1) + 'px';
    const left  = (Math.random() * 100).toFixed(1) + '%';
    const dur   = (Math.random() * 14 + 10).toFixed(1) + 's';
    const delay = (Math.random() * 12).toFixed(1) + 's';

    p.style.cssText = `--size:${size}; --dur:${dur}; --delay:${delay}; left:${left}; bottom:-10px;`;
    container.appendChild(p);
  }
})();

// ---- Email notify form ----
function handleSubmit(e) {
  e.preventDefault();
  const form   = e.target;
  const input  = form.querySelector('input[type="email"]');
  const thanks = document.getElementById('thanks');
  const email  = input.value.trim();

  if (!email) return;

  // Simulate submission (replace with real API call)
  form.querySelector('button').textContent = '...';
  setTimeout(() => {
    thanks.textContent = 'You\'re on the list. We\'ll be in touch.';
    input.value = '';
    form.querySelector('button').textContent = 'Notify Me';
    form.querySelector('button').disabled = true;
    form.querySelector('button').style.opacity = '0.5';
  }, 800);
}

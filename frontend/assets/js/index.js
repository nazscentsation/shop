/* ============================================================
   index.js — coming-soon page logic
   ============================================================ */
'use strict';

// ── If already authenticated, redirect to correct page ───────
(async () => {
  const [meRes, cfgRes] = await Promise.all([
    NS.api('GET', '/api/me'),
    NS.api('GET', '/api/config'),
  ]);
  if (!meRes.ok) return; // not logged in, stay on coming-soon
  const user       = meRes.data;
  const comingSoon = cfgRes.ok ? cfgRes.data.coming_soon : true;

  if (user.role === 'admin') {
    window.location.href = '/portal.html';
  } else if (!comingSoon) {
    // Shop is open — send customer to the shop
    window.location.href = '/shop.html';
  }
  // else: coming-soon mode, customer is logged in but shop not open yet — stay here
})();

// ── Notify form ───────────────────────────────────────────────
(function initNotify() {
  const form     = document.getElementById('notifyForm');
  const input    = document.getElementById('notifyEmail');
  const feedback = document.getElementById('notifyFeedback');
  if (!form) return;

  function setFeedback(msg, ok) {
    feedback.textContent = msg;
    feedback.className   = 'notify-feedback ' + (ok ? 'success' : 'error');
  }

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    const email = input.value.trim();
    if (!NS.validEmail(email)) { setFeedback('Please enter a valid email address.', false); return; }

    const btn = form.querySelector('button');
    NS.btnLoading(btn, true);

    const { ok, data } = await NS.api('POST', '/api/notify', { email });
    if (ok) {
      setFeedback("You're on the list. We'll be in touch.", true);
      input.value = '';
    } else {
      setFeedback(data.error || 'Something went wrong. Please try again.', false);
      NS.btnLoading(btn, false);
    }
  });
})();

// ── Hidden entry trigger ──────────────────────────────────────
// The `ns-dot` element requires 3 clicks within 2 s to open the modal.
// No obvious label or tooltip reveals its purpose.
(function initHiddenEntry() {
  const dot = document.getElementById('nsEntry');
  if (!dot) return;
  let clicks = 0, timer;

  dot.addEventListener('click', () => {
    clicks++;
    clearTimeout(timer);
    if (clicks >= 3) { clicks = 0; openModal(); return; }
    timer = setTimeout(() => { clicks = 0; }, 2000);
  });
})();

// ── Modal state ───────────────────────────────────────────────
const backdrop = document.getElementById('modalBackdrop');
const panels   = {
  login:    document.getElementById('panelLogin'),
  register: document.getElementById('panelRegister'),
  forgot:   document.getElementById('panelForgot'),
};

function openModal(panel = 'login') {
  backdrop.hidden = false;
  showPanel(panel);
  document.body.style.overflow = 'hidden';
}

function closeModal() {
  backdrop.hidden = true;
  document.body.style.overflow = '';
  clearFeedback();
}

function showPanel(name) {
  Object.values(panels).forEach(p => p.hidden = true);
  if (panels[name]) panels[name].hidden = false;
  clearFeedback();
}

function clearFeedback() {
  ['loginFeedback', 'registerFeedback', 'forgotFeedback'].forEach(id => {
    const el = document.getElementById(id);
    if (el) { el.textContent = ''; el.className = 'ns-modal-feedback'; }
  });
}

function setFeedback(id, msg, ok) {
  const el = document.getElementById(id);
  if (!el) return;
  el.textContent = msg;
  el.className   = 'ns-modal-feedback ' + (ok ? 'success' : 'error');
}

// Close controls
document.getElementById('modalClose').addEventListener('click', closeModal);
backdrop.addEventListener('click', (e) => { if (e.target === backdrop) closeModal(); });
document.addEventListener('keydown', (e) => { if (e.key === 'Escape') closeModal(); });

// Panel switchers
document.getElementById('toRegister').addEventListener('click', (e) => { e.preventDefault(); showPanel('register'); });
document.getElementById('toLogin').addEventListener('click',    (e) => { e.preventDefault(); showPanel('login'); });
document.getElementById('toForgot').addEventListener('click',   (e) => { e.preventDefault(); showPanel('forgot'); });
document.getElementById('forgotToLogin').addEventListener('click', (e) => { e.preventDefault(); showPanel('login'); });

// ── Login ─────────────────────────────────────────────────────
document.getElementById('loginForm').addEventListener('submit', async (e) => {
  e.preventDefault();
  const email = document.getElementById('loginEmail').value.trim();
  const pass  = document.getElementById('loginPass').value;
  const btn   = document.getElementById('loginBtn');

  if (!email || !pass) {
    setFeedback('loginFeedback', 'Please enter your email and password.', false);
    return;
  }

  NS.btnLoading(btn, true);

  const { ok, status, data } = await NS.api('POST', '/api/auth/login', { email, password: pass });

  if (ok) {
    NS.setUser(data.user);
    closeModal();
    // Brief delay so the cookie is set before redirect
    setTimeout(() => {
      window.location.href = data.user.role === 'admin' ? '/portal.html' : '/shop.html';
    }, 80);
    return;
  }

  NS.btnLoading(btn, false);

  if (status === 403) {
    // Email not verified — show specific, actionable message
    setFeedback('loginFeedback',
      'Your email address has not been verified. Please check your inbox or request a new link below.', false);
    return;
  }

  // For ALL other failures (wrong email, wrong password, locked, etc.)
  // use a single generic message per OWASP — never reveal which field is wrong
  setFeedback('loginFeedback',
    'Email or password is incorrect. Please try again.', false);

  // After 3 failed attempts tracked locally, surface the forgot-password link
  const attempts = parseInt(sessionStorage.getItem('ns_fa') || '0') + 1;
  sessionStorage.setItem('ns_fa', attempts);
  if (attempts >= 3) {
    const fb = document.getElementById('loginFeedback');
    fb.innerHTML = 'Email or password is incorrect. ' +
      '<a href="#" id="hintForgot" style="color:inherit;text-decoration:underline">Forgot your password?</a>';
    document.getElementById('hintForgot')?.addEventListener('click', (ev) => {
      ev.preventDefault(); showPanel('forgot');
    });
  }
});

// ── Register ──────────────────────────────────────────────────
document.getElementById('registerForm').addEventListener('submit', async (e) => {
  e.preventDefault();
  const first = document.getElementById('regFirst').value.trim();
  const last  = document.getElementById('regLast').value.trim();
  const email = document.getElementById('regEmail').value.trim();
  const pass  = document.getElementById('regPass').value;
  const btn   = document.getElementById('registerBtn');

  if (!first || !last) { setFeedback('registerFeedback', 'First and last name are required.', false); return; }
  if (!NS.validEmail(email)) { setFeedback('registerFeedback', 'Please enter a valid email address.', false); return; }
  if (pass.length < 8)  { setFeedback('registerFeedback', 'Password must be at least 8 characters.', false); return; }

  NS.btnLoading(btn, true);
  const { ok, data } = await NS.api('POST', '/api/auth/register', {
    first_name: first, last_name: last, email, password: pass,
  });
  NS.btnLoading(btn, false);

  if (ok) {
    setFeedback('registerFeedback',
      'Account created! Please check your email to verify your address before signing in.', true);
    document.getElementById('registerForm').reset();
  } else {
    setFeedback('registerFeedback', data.error || 'Could not create account. Please try again.', false);
  }
});

// ── Forgot password ───────────────────────────────────────────
document.getElementById('forgotForm').addEventListener('submit', async (e) => {
  e.preventDefault();
  const email = document.getElementById('forgotEmail').value.trim();
  const btn   = document.getElementById('forgotBtn');

  if (!NS.validEmail(email)) { setFeedback('forgotFeedback', 'Please enter a valid email address.', false); return; }

  NS.btnLoading(btn, true);
  await NS.api('POST', '/api/auth/forgot-password', { email });
  NS.btnLoading(btn, false);

  // Always show the same message — never confirm whether the email exists
  setFeedback('forgotFeedback',
    'If that address is registered, a reset link has been sent. Please check your inbox.', true);
  document.getElementById('forgotForm').reset();
});

/* ============================================================
   app.js — shared utilities for all Nazscentsation pages
   All API calls use credentials:'include' so the httpOnly
   cookie is sent automatically — the JWT never touches JS.
   ============================================================ */
'use strict';

const NS = (() => {

  // ── User session (role/name only — no token in JS) ──────────
  function getUser() {
    try { return JSON.parse(sessionStorage.getItem('ns_user') || 'null'); }
    catch { return null; }
  }
  function setUser(u) { sessionStorage.setItem('ns_user', JSON.stringify(u)); }
  function clearUser() { sessionStorage.removeItem('ns_user'); }

  // ── API wrapper ──────────────────────────────────────────────
  async function api(method, path, body) {
    const opts = {
      method,
      credentials: 'include',          // send httpOnly cookie automatically
      headers: { 'Content-Type': 'application/json' },
    };
    if (body !== undefined) opts.body = JSON.stringify(body);
    const res = await fetch(path, opts);
    let data;
    try { data = await res.json(); } catch { data = {}; }
    return { ok: res.ok, status: res.status, data };
  }

  // ── Auth guards ──────────────────────────────────────────────
  // Call on protected pages. Redirects to / if auth fails.
  async function requireAuth(role) {
    const res = await api('GET', '/api/me');
    if (!res.ok) { window.location.href = '/'; return null; }
    const user = res.data;
    if (role === 'admin' && user.role !== 'admin') {
      window.location.href = '/'; return null;
    }
    setUser(user);
    return user;
  }

  async function logout() {
    await api('POST', '/api/auth/logout');
    clearUser();
    window.location.href = '/';
  }

  // ── Toast notifications ──────────────────────────────────────
  function toast(msg, type = 'info') {
    const existing = document.getElementById('ns-toast');
    if (existing) existing.remove();

    const el = document.createElement('div');
    el.id = 'ns-toast';
    el.className = 'ns-toast ns-toast--' + type;
    el.textContent = msg;
    el.setAttribute('role', 'alert');
    document.body.appendChild(el);

    // Animate in
    requestAnimationFrame(() => el.classList.add('ns-toast--show'));

    // Auto-remove
    setTimeout(() => {
      el.classList.remove('ns-toast--show');
      setTimeout(() => el.remove(), 400);
    }, 4000);
  }

  // ── Helpers ──────────────────────────────────────────────────
  function formatCurrency(amount) {
    return new Intl.NumberFormat('en-NG', { style: 'currency', currency: 'NGN', minimumFractionDigits: 0 }).format(amount);
  }

  function formatDate(dateStr) {
    return new Intl.DateTimeFormat('en-GB', { day: 'numeric', month: 'long', year: 'numeric' }).format(new Date(dateStr));
  }

  function escapeHtml(str) {
    const d = document.createElement('div');
    d.appendChild(document.createTextNode(str));
    return d.innerHTML;
  }

  // Validate email format client-side (basic, server always re-validates)
  function validEmail(email) {
    return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.trim());
  }

  // Set button loading state
  function btnLoading(btn, loading) {
    btn.disabled = loading;
    if (loading) {
      btn.dataset.orig = btn.textContent;
      btn.textContent = '·  ·  ·';
    } else {
      btn.textContent = btn.dataset.orig || btn.textContent;
    }
  }

  return {
    api, getUser, setUser, clearUser, requireAuth,
    logout, toast,
    formatCurrency, formatDate, escapeHtml, validEmail, btnLoading,
  };
})();

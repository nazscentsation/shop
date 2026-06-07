/* ============================================================
   account.js — user profile, orders, tickets, password
   ============================================================ */
'use strict';

let currentUser = null;
let activeTicketId = null;

// ── Auth guard & init ─────────────────────────────────────────
(async () => {
  currentUser = await NS.requireAuth();
  if (!currentUser) return;

  populateSidebar(currentUser);
  document.getElementById('logoutBtn').addEventListener('click', NS.logout);
  document.getElementById('logoutSide').addEventListener('click', NS.logout);

  // Sidebar tab navigation
  document.querySelectorAll('[data-panel]').forEach(link => {
    link.addEventListener('click', e => {
      e.preventDefault();
      switchPanel(link.dataset.panel, link);
    });
  });

  // Load first active panel
  await loadPanel('profile');
})();

function populateSidebar(u) {
  document.getElementById('sidebarAvatar').textContent =
    (u.first_name?.[0] || '') + (u.last_name?.[0] || '');
  document.getElementById('sidebarName').textContent =
    `${u.first_name} ${u.last_name}`.trim() || 'Account';
  document.getElementById('sidebarEmail').textContent = u.email || '';
}

function switchPanel(name, linkEl) {
  document.querySelectorAll('.s-content-panel').forEach(p => p.classList.remove('active'));
  document.getElementById('panel' + name.charAt(0).toUpperCase() + name.slice(1))?.classList.add('active');
  document.querySelectorAll('[data-panel]').forEach(l => l.classList.remove('active'));
  linkEl?.classList.add('active');
  loadPanel(name);
}

async function loadPanel(name) {
  switch (name) {
    case 'profile':    await loadProfile();    break;
    case 'orders':     await loadOrders();     break;
    case 'enquiries':  await loadTickets();    break;
  }
}

// ── Profile ───────────────────────────────────────────────────
async function loadProfile() {
  const { ok, data } = await NS.api('GET', '/api/me');
  if (!ok) return;
  document.getElementById('pfFirst').value   = data.first_name  || '';
  document.getElementById('pfLast').value    = data.last_name   || '';
  document.getElementById('pfEmail').value   = data.email       || '';
  document.getElementById('pfPhone').value   = data.phone       || '';
  document.getElementById('pfAddress').value = data.address     || '';
  currentUser = data;
  populateSidebar(data);
}

document.getElementById('profileForm').addEventListener('submit', async e => {
  e.preventDefault();
  const btn = document.getElementById('profileBtn');
  const fb  = document.getElementById('profileFeedback');
  fb.className = 's-feedback';
  fb.textContent = '';

  NS.btnLoading(btn, true);
  const { ok, data } = await NS.api('PATCH', '/api/me', {
    first_name: document.getElementById('pfFirst').value.trim(),
    last_name:  document.getElementById('pfLast').value.trim(),
    phone:      document.getElementById('pfPhone').value.trim(),
    address:    document.getElementById('pfAddress').value.trim(),
  });
  NS.btnLoading(btn, false);

  if (ok) {
    fb.className = 's-feedback success';
    fb.textContent = 'Profile updated successfully.';
    const refreshed = await NS.api('GET', '/api/me');
    if (refreshed.ok) { currentUser = refreshed.data; NS.setUser(refreshed.data); populateSidebar(refreshed.data); }
  } else {
    fb.className = 's-feedback error';
    fb.textContent = data.error || 'Could not update profile. Please try again.';
  }
});

// ── Orders ────────────────────────────────────────────────────
async function loadOrders() {
  const tbody = document.getElementById('ordersBody');
  tbody.innerHTML = '<tr><td colspan="6" style="text-align:center;color:var(--text-soft);font-style:italic;padding:2rem">Loading…</td></tr>';

  const { ok, data } = await NS.api('GET', '/api/orders');
  if (!ok) { tbody.innerHTML = '<tr><td colspan="6" style="text-align:center;color:var(--danger)">Could not load orders.</td></tr>'; return; }

  if (!data.length) {
    tbody.innerHTML = '<tr><td colspan="6" style="text-align:center;color:var(--text-soft);font-style:italic;padding:2rem">No orders yet.</td></tr>';
    return;
  }

  tbody.innerHTML = data.map(o => `
    <tr>
      <td><strong>#${o.id}</strong></td>
      <td>${NS.formatDate(o.created_at)}</td>
      <td style="text-transform:capitalize">${NS.escapeHtml((o.payment_method || '').replace(/_/g,' ') || '—')}</td>
      <td><span class="s-badge s-badge--${o.status}">${o.status}</span></td>
      <td><strong>${NS.formatCurrency(o.total)}</strong></td>
      <td><button class="s-btn s-btn--ghost" style="padding:0.3rem 0.7rem;font-size:0.62rem" data-orderid="${o.id}">View</button></td>
    </tr>
  `).join('');

  tbody.querySelectorAll('[data-orderid]').forEach(btn => {
    btn.addEventListener('click', () => openOrderDetail(parseInt(btn.dataset.orderid)));
  });
}

async function openOrderDetail(id) {
  const modal   = document.getElementById('orderDetailModal');
  const content = document.getElementById('orderDetailContent');
  content.innerHTML = '<p style="text-align:center;color:var(--text-soft);font-style:italic;padding:2rem">Loading…</p>';
  modal.hidden = false;

  const { ok, data } = await NS.api('GET', `/api/orders/${id}`);
  if (!ok) { content.innerHTML = '<p style="color:var(--danger)">Could not load order.</p>'; return; }

  const items = (data.items || []).map(item => `
    <div style="display:flex;justify-content:space-between;padding:0.5rem 0;border-bottom:1px solid var(--border);font-size:0.82rem">
      <span>${NS.escapeHtml(item.product?.name || 'Item')} × ${item.quantity}</span>
      <span>${NS.formatCurrency(item.unit_price * item.quantity)}</span>
    </div>
  `).join('');

  content.innerHTML = `
    <div style="display:grid;grid-template-columns:1fr 1fr;gap:1.4rem;margin-bottom:1.4rem;font-size:0.82rem">
      <div>
        <p style="font-size:0.62rem;letter-spacing:0.18em;text-transform:uppercase;color:var(--text-soft);margin-bottom:0.4rem">Order Info</p>
        <p><strong>Order #${data.id}</strong></p>
        <p>Date: ${NS.formatDate(data.created_at)}</p>
        <p>Status: <span class="s-badge s-badge--${data.status}">${data.status}</span></p>
        <p style="margin-top:0.4rem;text-transform:capitalize">Payment: ${NS.escapeHtml((data.payment_method || '').replace(/_/g,' ') || '—')}</p>
      </div>
      <div>
        <p style="font-size:0.62rem;letter-spacing:0.18em;text-transform:uppercase;color:var(--text-soft);margin-bottom:0.4rem">Shipping Address</p>
        <p>${NS.escapeHtml(data.ship_name)}</p>
        <p>${NS.escapeHtml(data.ship_line1)}</p>
        ${data.ship_line2 ? `<p>${NS.escapeHtml(data.ship_line2)}</p>` : ''}
        <p>${NS.escapeHtml(data.ship_city)}, ${NS.escapeHtml(data.ship_postal)}</p>
        <p>${NS.escapeHtml(data.ship_country)}</p>
      </div>
    </div>
    <p style="font-size:0.62rem;letter-spacing:0.18em;text-transform:uppercase;color:var(--text-soft);margin-bottom:0.6rem">Items</p>
    ${items}
    <div style="display:flex;justify-content:space-between;padding:0.8rem 0;font-size:0.82rem">
      <strong>Total</strong>
      <strong>${NS.formatCurrency(data.total)}</strong>
    </div>
  `;
}

document.getElementById('orderDetailClose').addEventListener('click', () => {
  document.getElementById('orderDetailModal').hidden = true;
});
document.getElementById('orderDetailModal').addEventListener('click', e => {
  if (e.target === document.getElementById('orderDetailModal'))
    document.getElementById('orderDetailModal').hidden = true;
});

// ── Tickets ───────────────────────────────────────────────────
async function loadTickets() {
  const tbody = document.getElementById('ticketsBody');
  tbody.innerHTML = '<tr><td colspan="5" style="text-align:center;color:var(--text-soft);font-style:italic;padding:2rem">Loading…</td></tr>';

  const { ok, data } = await NS.api('GET', '/api/tickets');
  if (!ok) { tbody.innerHTML = '<tr><td colspan="5" style="text-align:center;color:var(--danger)">Could not load enquiries.</td></tr>'; return; }

  // Update badge
  const openCount = data.filter(t => t.status === 'open').length;
  const badge = document.getElementById('openTicketsBadge');
  if (openCount > 0) { badge.textContent = openCount; badge.hidden = false; } else { badge.hidden = true; }

  if (!data.length) {
    tbody.innerHTML = '<tr><td colspan="5" style="text-align:center;color:var(--text-soft);font-style:italic;padding:2rem">No enquiries yet. Create one using the button above.</td></tr>';
    return;
  }

  tbody.innerHTML = data.map(t => `
    <tr>
      <td><strong>#${t.id}</strong></td>
      <td>${NS.escapeHtml(t.subject)}</td>
      <td><span class="s-badge s-badge--${t.status}">${t.status}</span></td>
      <td>${NS.formatDate(t.updated_at)}</td>
      <td><button class="s-btn s-btn--ghost" style="padding:0.3rem 0.7rem;font-size:0.62rem" data-tid="${t.id}">View Thread</button></td>
    </tr>
  `).join('');

  tbody.querySelectorAll('[data-tid]').forEach(btn => {
    btn.addEventListener('click', () => openTicketThread(parseInt(btn.dataset.tid)));
  });
}

// New ticket modal
document.getElementById('newTicketBtn').addEventListener('click', () => {
  document.getElementById('newTicketModal').hidden = false;
  document.getElementById('newTicketFeedback').textContent = '';
});
document.getElementById('newTicketClose').addEventListener('click', () => {
  document.getElementById('newTicketModal').hidden = true;
});
document.getElementById('newTicketModal').addEventListener('click', e => {
  if (e.target === document.getElementById('newTicketModal'))
    document.getElementById('newTicketModal').hidden = true;
});

document.getElementById('submitTicketBtn').addEventListener('click', async () => {
  const subject = document.getElementById('ticketSubject').value.trim();
  const message = document.getElementById('ticketMessage').value.trim();
  const fb      = document.getElementById('newTicketFeedback');
  const btn     = document.getElementById('submitTicketBtn');

  fb.className = 's-feedback error';
  if (!subject) { fb.textContent = 'Please enter a subject.'; return; }
  if (!message) { fb.textContent = 'Please enter a message.'; return; }

  NS.btnLoading(btn, true);
  const { ok, data } = await NS.api('POST', '/api/tickets', { subject, message });
  NS.btnLoading(btn, false);

  if (ok) {
    document.getElementById('newTicketModal').hidden = true;
    document.getElementById('ticketSubject').value = '';
    document.getElementById('ticketMessage').value = '';
    NS.toast('Enquiry submitted. We\'ll respond shortly.', 'success');
    await loadTickets();
  } else {
    fb.textContent = data.error || 'Could not submit enquiry. Please try again.';
  }
});

async function openTicketThread(id) {
  activeTicketId = id;
  const modal = document.getElementById('ticketThreadModal');
  const msgs  = document.getElementById('threadMessages');
  msgs.innerHTML = '<p style="text-align:center;color:var(--text-soft);font-style:italic">Loading…</p>';
  modal.hidden = false;

  const { ok, data } = await NS.api('GET', `/api/tickets/${id}`);
  if (!ok) { msgs.innerHTML = '<p style="color:var(--danger)">Could not load thread.</p>'; return; }

  document.getElementById('threadTitle').textContent = `#${data.id}: ${data.subject}`;
  const statusEl = document.getElementById('threadStatus');
  statusEl.innerHTML = `<span class="s-badge s-badge--${data.status}">${data.status}</span>`;

  const replyWrap = document.getElementById('threadReplyWrap');
  replyWrap.style.display = data.status === 'closed' ? 'none' : 'block';

  msgs.innerHTML = (data.messages || []).map(m => `
    <div class="s-msg s-msg--${m.sender}">
      <p>${NS.escapeHtml(m.body)}</p>
      <p class="s-msg-meta">${m.sender === 'admin' ? 'Support Team' : 'You'} · ${NS.formatDate(m.created_at)}</p>
    </div>
  `).join('') || '<p style="text-align:center;color:var(--text-soft);font-style:italic">No messages yet.</p>';

  msgs.scrollTop = msgs.scrollHeight;
}

document.getElementById('threadClose').addEventListener('click', () => {
  document.getElementById('ticketThreadModal').hidden = true;
});
document.getElementById('ticketThreadModal').addEventListener('click', e => {
  if (e.target === document.getElementById('ticketThreadModal'))
    document.getElementById('ticketThreadModal').hidden = true;
});

document.getElementById('sendReplyBtn').addEventListener('click', async () => {
  const body = document.getElementById('threadReply').value.trim();
  const fb   = document.getElementById('threadFeedback');
  const btn  = document.getElementById('sendReplyBtn');

  fb.className = 's-feedback error';
  if (!body) { fb.textContent = 'Please type a reply.'; return; }

  NS.btnLoading(btn, true);
  const { ok, data } = await NS.api('POST', `/api/tickets/${activeTicketId}/reply`, { body });
  NS.btnLoading(btn, false);

  if (ok) {
    document.getElementById('threadReply').value = '';
    fb.textContent = '';
    await openTicketThread(activeTicketId);
  } else {
    fb.textContent = data.error || 'Could not send reply.';
  }
});

// ── Change password ───────────────────────────────────────────
document.getElementById('passwordForm').addEventListener('submit', async e => {
  e.preventDefault();
  const cur     = document.getElementById('pwCurrent').value;
  const newPass = document.getElementById('pwNew').value;
  const confirm = document.getElementById('pwConfirm').value;
  const fb      = document.getElementById('passwordFeedback');
  const btn     = document.getElementById('passwordBtn');

  fb.className = 's-feedback error';
  fb.textContent = '';

  if (!cur)              { fb.textContent = 'Please enter your current password.'; return; }
  if (newPass.length < 8){ fb.textContent = 'New password must be at least 8 characters.'; return; }
  if (newPass !== confirm){ fb.textContent = 'New passwords do not match.'; return; }

  NS.btnLoading(btn, true);
  const { ok, data } = await NS.api('PATCH', '/api/me/password', {
    current_password: cur,
    new_password:     newPass,
  });
  NS.btnLoading(btn, false);

  if (ok) {
    fb.className = 's-feedback success';
    fb.textContent = 'Password changed successfully.';
    document.getElementById('passwordForm').reset();
  } else {
    fb.textContent = data.error || 'Could not change password. Please try again.';
  }
});

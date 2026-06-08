/* ============================================================
   portal.js — admin management panel
   ============================================================ */
'use strict';

let adminUser = null;

// Format product ID as NS-000001
function nsId(id) { return 'NS-' + String(id).padStart(6, '0'); }

// ── Auth guard (admin only) ───────────────────────────────────
(async () => {
  adminUser = await NS.requireAuth('admin');
  if (!adminUser) return;

  document.getElementById('portalAdminName').textContent =
    `${adminUser.first_name} ${adminUser.last_name}`.trim();
  document.getElementById('portalLogout').addEventListener('click', NS.logout);

  // Sidebar nav
  document.querySelectorAll('[data-panel]').forEach(link => {
    link.addEventListener('click', e => {
      e.preventDefault();
      switchPanel(link.dataset.panel, link);
    });
  });

  await loadDashboard();
})();

// ── Panel switching ───────────────────────────────────────────
const PANEL_TITLES = {
  dashboard:   'Dashboard',
  products:    'Products',
  orders:      'Orders',
  tickets:     'Support Tickets',
  subscribers: 'Email Subscribers',
  users:       'Customers',
};

function switchPanel(name, linkEl) {
  document.querySelectorAll('.s-portal-panel').forEach(p => p.classList.remove('active'));
  document.getElementById('panel' + name.charAt(0).toUpperCase() + name.slice(1))?.classList.add('active');
  document.querySelectorAll('[data-panel]').forEach(l => l.classList.remove('active'));
  linkEl?.classList.add('active');
  document.getElementById('portalPageTitle').textContent = PANEL_TITLES[name] || '';

  switch (name) {
    case 'dashboard':   loadDashboard();   break;
    case 'products':    loadProducts();    break;
    case 'orders':      loadOrders();      break;
    case 'tickets':     loadTickets();     break;
    case 'subscribers': loadSubscribers(); break;
    case 'users':       loadUsers();       break;
  }
}

// ── Dashboard ─────────────────────────────────────────────────
async function loadDashboard() {
  const [products, orders, tickets, subs, users] = await Promise.all([
    NS.api('GET', '/api/admin/products'),
    NS.api('GET', '/api/admin/orders'),
    NS.api('GET', '/api/admin/tickets'),
    NS.api('GET', '/api/admin/notify'),
    NS.api('GET', '/api/admin/users'),
  ]);

  document.getElementById('statProducts').textContent = products.ok ? (products.data.length || 0) : '—';
  document.getElementById('statOrders').textContent   = orders.ok   ? (orders.data.length   || 0) : '—';
  document.getElementById('statSubs').textContent     = subs.ok     ? (subs.data.length     || 0) : '—';
  document.getElementById('statUsers').textContent    = users.ok    ? (users.data.length    || 0) : '—';

  const openTickets = tickets.ok ? tickets.data.filter(t => t.status === 'open') : [];
  document.getElementById('statTickets').textContent = openTickets.length;
  updateTicketBadge(openTickets.length);

  // Recent orders (last 5)
  const tbody = document.getElementById('dashRecentOrders');
  if (orders.ok && orders.data.length) {
    tbody.innerHTML = orders.data.slice(0, 5).map(o => `
      <tr>
        <td>#${o.id}</td>
        <td>${NS.escapeHtml(o.first_name + ' ' + o.last_name)}</td>
        <td><strong>${NS.formatCurrency(o.total)}</strong></td>
        <td><span class="s-badge s-badge--${o.status}">${o.status}</span></td>
        <td>${NS.formatDate(o.created_at)}</td>
      </tr>`).join('');
  } else {
    tbody.innerHTML = '<tr><td colspan="5" style="text-align:center;color:var(--text-soft);font-style:italic;padding:1.5rem">No orders yet.</td></tr>';
  }
}

function updateTicketBadge(count) {
  const el = document.getElementById('openTicketCount');
  if (count > 0) { el.textContent = count; el.hidden = false; }
  else el.hidden = true;
}

// ── Products ──────────────────────────────────────────────────
async function loadProducts() {
  const tbody = document.getElementById('productsBody');
  tbody.innerHTML = loadingRow(8);

  const { ok, data } = await NS.api('GET', '/api/admin/products');
  if (!ok) { tbody.innerHTML = errorRow(9); return; }

  if (!data.length) { tbody.innerHTML = emptyRow(9, 'No products yet.'); return; }

  tbody.innerHTML = data.map(p => `
    <tr>
      <td style="font-size:0.7rem;color:var(--text-soft);white-space:nowrap">${nsId(p.id)}</td>
      <td>
        ${p.image_url
          ? `<img src="${NS.escapeHtml(p.image_url)}" class="s-img-preview" alt="${NS.escapeHtml(p.name)}"/>`
          : `<div class="s-img-preview" style="display:flex;align-items:center;justify-content:center;color:var(--text-soft);font-size:0.62rem">No img</div>`}
      </td>
      <td><strong>${NS.escapeHtml(p.name)}</strong></td>
      <td>${NS.escapeHtml(p.category || '—')}</td>
      <td>${NS.formatCurrency(p.price)}</td>
      <td>${p.discount_pct > 0 ? `<span class="s-badge s-badge--pending">${p.discount_pct}%</span>` : '—'}</td>
      <td>${p.stock}</td>
      <td><span class="s-badge ${p.active ? 's-badge--paid' : 's-badge--closed'}">${p.active ? 'Active' : 'Hidden'}</span></td>
      <td style="white-space:nowrap">
        <button class="s-btn s-btn--ghost s-btn--icon" data-edit="${p.id}" title="Edit">&#9998;</button>
        <button class="s-btn s-btn--danger s-btn--icon" data-delete="${p.id}" title="Delete" style="margin-left:4px">&#128465;</button>
      </td>
    </tr>
  `).join('');

  tbody.querySelectorAll('[data-edit]').forEach(btn =>
    btn.addEventListener('click', () => openProductForm(parseInt(btn.dataset.edit), data)));
  tbody.querySelectorAll('[data-delete]').forEach(btn =>
    btn.addEventListener('click', () => deleteProduct(parseInt(btn.dataset.delete))));
}

// Add product button
document.getElementById('addProductBtn').addEventListener('click', () => openProductForm(null, []));

function openProductForm(id, products) {
  const isEdit = id !== null;
  document.getElementById('productFormTitle').textContent = isEdit ? 'Edit Product' : 'Add Product';
  document.getElementById('productFormId').value = id || '';
  document.getElementById('productFormFeedback').textContent = '';
  document.getElementById('pActiveWrap').hidden = !isEdit;

  if (isEdit) {
    const p = products.find(x => x.id === id);
    if (p) {
      document.getElementById('pName').value     = p.name;
      document.getElementById('pDesc').value     = p.description || '';
      document.getElementById('pPrice').value    = p.price;
      document.getElementById('pDiscount').value = p.discount_pct || 0;
      document.getElementById('pStock').value    = p.stock;
      document.getElementById('pCategory').value = p.category || '';
      document.getElementById('pImage').value    = p.image_url || '';
      document.getElementById('pNotes').value    = (p.notes || []).join(', ');
      document.getElementById('pActive').checked = p.active;
      updateImgPreview(p.image_url);
    }
  } else {
    document.getElementById('pName').value     = '';
    document.getElementById('pDesc').value     = '';
    document.getElementById('pPrice').value    = '';
    document.getElementById('pDiscount').value = '0';
    document.getElementById('pStock').value    = '0';
    document.getElementById('pCategory').value = '';
    document.getElementById('pImage').value    = '';
    document.getElementById('pNotes').value    = '';
    updateImgPreview('');
  }

  document.getElementById('productFormModal').hidden = false;
}

document.getElementById('pImage').addEventListener('input', e => updateImgPreview(e.target.value));

function updateImgPreview(url) {
  const prev = document.getElementById('pImagePreview');
  if (url) { prev.src = url; prev.style.display = 'block'; }
  else prev.style.display = 'none';
}

document.getElementById('productFormClose').addEventListener('click', () => {
  document.getElementById('productFormModal').hidden = true;
});
document.getElementById('productFormModal').addEventListener('click', e => {
  if (e.target === document.getElementById('productFormModal'))
    document.getElementById('productFormModal').hidden = true;
});

document.getElementById('saveProductBtn').addEventListener('click', async () => {
  const id   = document.getElementById('productFormId').value;
  const fb   = document.getElementById('productFormFeedback');
  const btn  = document.getElementById('saveProductBtn');
  fb.className = 's-feedback error'; fb.textContent = '';

  const name  = document.getElementById('pName').value.trim();
  const price = parseFloat(document.getElementById('pPrice').value);
  if (!name)        { fb.textContent = 'Product name is required.'; return; }
  if (isNaN(price) || price <= 0) { fb.textContent = 'Please enter a valid price.'; return; }

  const notes = document.getElementById('pNotes').value
    .split(',').map(s => s.trim()).filter(Boolean);

  const payload = {
    name,
    description:  document.getElementById('pDesc').value.trim(),
    price,
    discount_pct: parseInt(document.getElementById('pDiscount').value) || 0,
    stock:        parseInt(document.getElementById('pStock').value)    || 0,
    image_url:    document.getElementById('pImage').value.trim(),
    category:     document.getElementById('pCategory').value.trim(),
    notes,
  };

  NS.btnLoading(btn, true);
  let res;
  if (id) {
    payload.active = document.getElementById('pActive').checked;
    res = await NS.api('PATCH', `/api/admin/products/${id}`, payload);
  } else {
    res = await NS.api('POST', '/api/admin/products', payload);
  }
  NS.btnLoading(btn, false);

  if (res.ok) {
    document.getElementById('productFormModal').hidden = true;
    NS.toast(id ? 'Product updated.' : 'Product added.', 'success');
    loadProducts();
  } else {
    fb.textContent = res.data.error || 'Could not save product.';
  }
});

// ── Product preview ───────────────────────────────────────────
document.getElementById('previewProductBtn').addEventListener('click', () => {
  const name     = document.getElementById('pName').value.trim() || 'Product Name';
  const price    = parseFloat(document.getElementById('pPrice').value) || 0;
  const discount = parseInt(document.getElementById('pDiscount').value) || 0;
  const stock    = parseInt(document.getElementById('pStock').value) || 0;
  const imageUrl = document.getElementById('pImage').value.trim();
  const category = document.getElementById('pCategory').value.trim();
  const ep       = discount > 0 ? price * (1 - discount / 100) : price;

  const priceHtml = discount > 0
    ? `<span class="s-price s-price-sale">${NS.formatCurrency(ep)}</span>
       <span class="s-price-original">${NS.formatCurrency(price)}</span>`
    : `<span class="s-price">${NS.formatCurrency(price)}</span>`;

  document.getElementById('productPreviewCard').innerHTML = `
    <div class="s-card" style="pointer-events:none">
      <div class="s-card-img-wrap">
        ${imageUrl
          ? `<img class="s-card-img" src="${NS.escapeHtml(imageUrl)}" alt="${NS.escapeHtml(name)}"/>`
          : `<div class="s-card-placeholder">No Image</div>`}
        ${discount > 0 ? `<span class="s-badge-discount">${discount}% off</span>` : ''}
      </div>
      <div class="s-card-body">
        ${category ? `<p class="s-card-category">${NS.escapeHtml(category)}</p>` : ''}
        <h3 class="s-card-name">${NS.escapeHtml(name)}</h3>
        <div class="s-card-price-row">${priceHtml}</div>
        <p style="font-size:0.68rem;color:var(--text-soft);margin-top:0.4rem">
          ${stock > 0 ? `${stock} in stock` : '<span style="color:var(--danger)">Out of stock</span>'}
        </p>
      </div>
    </div>`;

  document.getElementById('productFormModal').hidden = true;
  document.getElementById('productPreviewModal').hidden = false;
});

document.getElementById('productPreviewClose').addEventListener('click', () => {
  document.getElementById('productPreviewModal').hidden = true;
  document.getElementById('productFormModal').hidden = false;
});
document.getElementById('previewBackBtn').addEventListener('click', () => {
  document.getElementById('productPreviewModal').hidden = true;
  document.getElementById('productFormModal').hidden = false;
});
document.getElementById('previewSaveBtn').addEventListener('click', () => {
  document.getElementById('productPreviewModal').hidden = true;
  document.getElementById('productFormModal').hidden = false;
  document.getElementById('saveProductBtn').click();
});

async function deleteProduct(id) {
  if (!confirm('Remove this product from the shop?')) return;
  const { ok } = await NS.api('DELETE', `/api/admin/products/${id}`);
  if (ok) { NS.toast('Product removed.', 'success'); loadProducts(); }
  else NS.toast('Could not remove product.', 'error');
}

// ── Orders ────────────────────────────────────────────────────
async function loadOrders() {
  const tbody = document.getElementById('adminOrdersBody');
  tbody.innerHTML = loadingRow(8);

  const { ok, data } = await NS.api('GET', '/api/admin/orders');
  if (!ok) { tbody.innerHTML = errorRow(8); return; }

  if (!data.length) { tbody.innerHTML = emptyRow(8, 'No orders yet.'); return; }

  tbody.innerHTML = data.map(o => `
    <tr>
      <td><strong>#${o.id}</strong></td>
      <td>${NS.escapeHtml((o.first_name + ' ' + o.last_name).trim() || 'Guest')}<br/>
          <span style="font-size:0.7rem;color:var(--text-soft)">${NS.escapeHtml(o.user_email)}</span></td>
      <td style="font-size:0.78rem">${NS.escapeHtml(o.ship_city || '—')}${o.ship_country ? ', ' + NS.escapeHtml(o.ship_country) : ''}</td>
      <td style="text-transform:capitalize">${NS.escapeHtml((o.payment_method || '').replace(/_/g,' ') || '—')}</td>
      <td><strong>${NS.formatCurrency(o.total)}</strong></td>
      <td><span class="s-badge s-badge--${o.status}">${o.status}</span></td>
      <td>${NS.formatDate(o.created_at)}</td>
      <td>
        <button class="s-btn s-btn--ghost s-btn--icon" data-orderidx="${o.id}" title="View &amp; Update">&#9998;</button>
      </td>
    </tr>
  `).join('');

  // Store order data on the rows for the modal
  const ordersMap = Object.fromEntries(data.map(o => [o.id, o]));

  tbody.querySelectorAll('[data-orderidx]').forEach(btn => {
    btn.addEventListener('click', () => {
      const o = ordersMap[parseInt(btn.dataset.orderidx)];
      if (!o) return;
      document.getElementById('orderStatusId').value = o.id;
      document.getElementById('orderStatusLabel').textContent = `Order #${o.id}`;
      document.getElementById('orderStatusSelect').value = o.status;
      document.getElementById('orderStatusFeedback').textContent = '';

      // Populate detail fields
      const name = (o.first_name + ' ' + o.last_name).trim() || 'Guest';
      document.getElementById('odCustomer').textContent = name;
      document.getElementById('odEmail').textContent = o.user_email || '';
      document.getElementById('odPayment').textContent = (o.payment_method || '').replace(/_/g, ' ') || '—';
      document.getElementById('odTotal').textContent = NS.formatCurrency(o.total);

      const addrParts = [
        o.ship_name,
        o.ship_line1,
        o.ship_line2,
        [o.ship_city, o.ship_postal].filter(Boolean).join(' '),
        o.ship_country,
      ].filter(Boolean);
      document.getElementById('odAddress').innerHTML = addrParts.map(p => NS.escapeHtml(p)).join('<br/>');

      document.getElementById('orderStatusModal').hidden = false;
    });
  });
}

document.getElementById('orderStatusClose').addEventListener('click', () => {
  document.getElementById('orderStatusModal').hidden = true;
});
document.getElementById('orderStatusModal').addEventListener('click', e => {
  if (e.target === document.getElementById('orderStatusModal'))
    document.getElementById('orderStatusModal').hidden = true;
});

document.getElementById('saveOrderStatusBtn').addEventListener('click', async () => {
  const id     = document.getElementById('orderStatusId').value;
  const status = document.getElementById('orderStatusSelect').value;
  const fb     = document.getElementById('orderStatusFeedback');
  const btn    = document.getElementById('saveOrderStatusBtn');

  NS.btnLoading(btn, true);
  const { ok, data } = await NS.api('PATCH', `/api/admin/orders/${id}/status`, { status });
  NS.btnLoading(btn, false);

  if (ok) {
    document.getElementById('orderStatusModal').hidden = true;
    NS.toast('Order status updated.', 'success');
    loadOrders();
  } else {
    fb.className = 's-feedback error';
    fb.textContent = data.error || 'Could not update status.';
  }
});

// ── Tickets ───────────────────────────────────────────────────
async function loadTickets() {
  const tbody = document.getElementById('adminTicketsBody');
  tbody.innerHTML = loadingRow(6);

  const { ok, data } = await NS.api('GET', '/api/admin/tickets');
  if (!ok) { tbody.innerHTML = errorRow(6); return; }

  if (!data.length) { tbody.innerHTML = emptyRow(6, 'No tickets yet.'); return; }

  updateTicketBadge(data.filter(t => t.status === 'open').length);

  tbody.innerHTML = data.map(t => `
    <tr>
      <td><strong>#${t.id}</strong></td>
      <td>${NS.escapeHtml(t.first_name + ' ' + t.last_name)}<br/>
          <span style="font-size:0.7rem;color:var(--text-soft)">${NS.escapeHtml(t.user_email)}</span></td>
      <td>${NS.escapeHtml(t.subject)}</td>
      <td><span class="s-badge s-badge--${t.status}">${t.status}</span></td>
      <td>${NS.formatDate(t.updated_at)}</td>
      <td>
        <button class="s-btn s-btn--ghost s-btn--icon" data-tid="${t.id}" data-status="${t.status}" title="View &amp; Reply">&#128172;</button>
      </td>
    </tr>
  `).join('');

  tbody.querySelectorAll('[data-tid]').forEach(btn => {
    btn.addEventListener('click', () => openAdminTicketThread(parseInt(btn.dataset.tid)));
  });
}

async function openAdminTicketThread(id) {
  const modal = document.getElementById('ticketReplyModal');
  const msgs  = document.getElementById('adminThreadMessages');
  msgs.innerHTML = '<p style="text-align:center;color:var(--text-soft);font-style:italic">Loading…</p>';
  document.getElementById('adminThreadId').value = id;
  document.getElementById('adminReplyFeedback').textContent = '';
  modal.hidden = false;

  const { ok, data } = await NS.api('GET', `/api/admin/tickets/${id}`);
  if (!ok) { msgs.innerHTML = '<p style="color:var(--danger)">Could not load ticket.</p>'; return; }

  document.getElementById('adminThreadTitle').textContent = `#${data.id}: ${data.subject}`;
  document.getElementById('adminThreadMeta').textContent  =
    `From: ${data.messages?.[0] ? '' : ''}Ticket #${data.id}`;
  document.getElementById('adminThreadStatus').innerHTML =
    `<span class="s-badge s-badge--${data.status}">${data.status}</span>`;

  const toggleBtn = document.getElementById('toggleTicketStatusBtn');
  toggleBtn.textContent = data.status === 'open' ? 'Close ticket' : 'Reopen ticket';
  toggleBtn.onclick = () => toggleTicketStatus(id, data.status);

  msgs.innerHTML = (data.messages || []).map(m => `
    <div class="s-msg s-msg--${m.sender}">
      <p>${NS.escapeHtml(m.body)}</p>
      <p class="s-msg-meta">${m.sender === 'admin' ? 'Support Team' : 'Customer'} · ${NS.formatDate(m.created_at)}</p>
    </div>
  `).join('') || '<p style="text-align:center;color:var(--text-soft);font-style:italic">No messages.</p>';

  msgs.scrollTop = msgs.scrollHeight;
}

async function toggleTicketStatus(id, current) {
  const newStatus = current === 'open' ? 'closed' : 'open';
  const { ok } = await NS.api('PATCH', `/api/admin/tickets/${id}/status`, { status: newStatus });
  if (ok) {
    NS.toast(`Ticket ${newStatus}.`, 'success');
    document.getElementById('ticketReplyModal').hidden = true;
    loadTickets();
  } else NS.toast('Could not update ticket status.', 'error');
}

document.getElementById('ticketReplyClose').addEventListener('click', () => {
  document.getElementById('ticketReplyModal').hidden = true;
});
document.getElementById('ticketReplyModal').addEventListener('click', e => {
  if (e.target === document.getElementById('ticketReplyModal'))
    document.getElementById('ticketReplyModal').hidden = true;
});

document.getElementById('sendAdminReplyBtn').addEventListener('click', async () => {
  const id   = document.getElementById('adminThreadId').value;
  const body = document.getElementById('adminReplyBody').value.trim();
  const fb   = document.getElementById('adminReplyFeedback');
  const btn  = document.getElementById('sendAdminReplyBtn');

  fb.className = 's-feedback error';
  if (!body) { fb.textContent = 'Please type a reply.'; return; }

  NS.btnLoading(btn, true);
  const { ok, data } = await NS.api('POST', `/api/admin/tickets/${id}/reply`, { body });
  NS.btnLoading(btn, false);

  if (ok) {
    document.getElementById('adminReplyBody').value = '';
    fb.textContent = '';
    NS.toast('Reply sent.', 'success');
    // Reload thread to show new message
    const { data: ticket } = await NS.api('GET', `/api/admin/tickets/${id}`);
    if (ticket) {
      const msgs = document.getElementById('adminThreadMessages');
      msgs.innerHTML = (ticket.messages || []).map(m => `
        <div class="s-msg s-msg--${m.sender}">
          <p>${NS.escapeHtml(m.body)}</p>
          <p class="s-msg-meta">${m.sender === 'admin' ? 'Support Team' : 'Customer'} · ${NS.formatDate(m.created_at)}</p>
        </div>
      `).join('');
      msgs.scrollTop = msgs.scrollHeight;
    }
  } else {
    fb.textContent = data.error || 'Could not send reply.';
  }
});

// ── Subscribers ───────────────────────────────────────────────
async function loadSubscribers() {
  const tbody = document.getElementById('subscribersBody');
  tbody.innerHTML = loadingRow(2);

  const { ok, data } = await NS.api('GET', '/api/admin/notify');
  if (!ok) { tbody.innerHTML = errorRow(2); return; }

  document.getElementById('subCount').textContent =
    data.length ? `${data.length} subscriber${data.length !== 1 ? 's' : ''}` : '';

  if (!data.length) { tbody.innerHTML = emptyRow(2, 'No subscribers yet.'); return; }

  tbody.innerHTML = data.map(s => `
    <tr>
      <td>${NS.escapeHtml(s.email)}</td>
      <td>${NS.formatDate(s.created_at)}</td>
    </tr>
  `).join('');
}

// ── Customers ─────────────────────────────────────────────────
async function loadUsers() {
  const tbody = document.getElementById('usersBody');
  tbody.innerHTML = loadingRow(6);

  const { ok, data } = await NS.api('GET', '/api/admin/users');
  if (!ok) { tbody.innerHTML = errorRow(6); return; }

  if (!data.length) { tbody.innerHTML = emptyRow(6, 'No customers yet.'); return; }

  tbody.innerHTML = data.map(u => `
    <tr>
      <td>${NS.escapeHtml(u.first_name + ' ' + u.last_name)}</td>
      <td>${NS.escapeHtml(u.email)}</td>
      <td>${NS.escapeHtml(u.phone || '—')}</td>
      <td><span class="s-badge ${u.role === 'admin' ? 's-badge--shipped' : 's-badge--closed'}">${u.role}</span></td>
      <td>${u.email_verified
        ? '<span class="s-badge s-badge--paid">Verified</span>'
        : '<span class="s-badge s-badge--pending">Unverified</span>'}</td>
      <td>${NS.formatDate(u.created_at)}</td>
    </tr>
  `).join('');
}

// ── Create user modal ─────────────────────────────────────────
document.getElementById('newUserBtn').addEventListener('click', () => {
  document.getElementById('createUserFeedback').hidden = true;
  document.getElementById('cuFirst').value = '';
  document.getElementById('cuLast').value  = '';
  document.getElementById('cuEmail').value = '';
  document.getElementById('cuPass').value  = '';
  document.getElementById('cuRole').value  = 'customer';
  document.getElementById('createUserModal').hidden = false;
});

function closeCreateUserModal() { document.getElementById('createUserModal').hidden = true; }
document.getElementById('createUserClose').addEventListener('click', closeCreateUserModal);
document.getElementById('createUserCancel').addEventListener('click', closeCreateUserModal);
document.getElementById('createUserModal').addEventListener('click', e => {
  if (e.target === document.getElementById('createUserModal')) closeCreateUserModal();
});

document.getElementById('createUserSave').addEventListener('click', async () => {
  const fb   = document.getElementById('createUserFeedback');
  const btn  = document.getElementById('createUserSave');
  const first = document.getElementById('cuFirst').value.trim();
  const last  = document.getElementById('cuLast').value.trim();
  const email = document.getElementById('cuEmail').value.trim();
  const pass  = document.getElementById('cuPass').value;
  const role  = document.getElementById('cuRole').value;

  fb.hidden = true;
  if (!first || !last || !email || !pass) {
    fb.textContent = 'All fields are required.'; fb.hidden = false; return;
  }
  if (pass.length < 8) {
    fb.textContent = 'Password must be at least 8 characters.'; fb.hidden = false; return;
  }

  NS.btnLoading(btn, true);
  const { ok, data } = await NS.api('POST', '/api/admin/users', { first_name: first, last_name: last, email, password: pass, role });
  NS.btnLoading(btn, false);

  if (ok) {
    closeCreateUserModal();
    NS.toast(`Account created for ${first} ${last}`, 'success');
    loadUsers();
  } else {
    fb.textContent = data.error || 'Could not create account.';
    fb.hidden = false;
  }
});

// ── Table helpers ─────────────────────────────────────────────
function loadingRow(cols) {
  return `<tr><td colspan="${cols}" style="text-align:center;color:var(--text-soft);font-style:italic;padding:2rem">Loading…</td></tr>`;
}
function errorRow(cols) {
  return `<tr><td colspan="${cols}" style="text-align:center;color:var(--danger);padding:2rem">Could not load data. Please refresh.</td></tr>`;
}
function emptyRow(cols, msg) {
  return `<tr><td colspan="${cols}" style="text-align:center;color:var(--text-soft);font-style:italic;padding:2rem">${msg}</td></tr>`;
}

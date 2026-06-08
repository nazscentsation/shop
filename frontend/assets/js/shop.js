/* ============================================================
   shop.js — product listing, cart, checkout
   ============================================================ */
'use strict';

let currentUser   = null;
let allProducts   = [];
let currentPage   = 1;
let totalProducts = 0;
let categories    = new Set();
let activeCategory = '';
const PAGE_SIZE   = 12;

// Cart state — keyed by product id
const cart = (() => {
  let items = {};
  const save  = () => localStorage.setItem('ns_cart', JSON.stringify(items));
  const load  = () => { try { items = JSON.parse(localStorage.getItem('ns_cart') || '{}'); } catch { items = {}; } };
  load();
  return {
    get items()  { return items; },
    add(product) {
      if (items[product.id]) {
        items[product.id].qty = Math.min(items[product.id].qty + 1, product.stock);
      } else {
        items[product.id] = { product, qty: 1 };
      }
      save(); renderCart();
    },
    remove(id)   { delete items[id]; save(); renderCart(); },
    setQty(id, q) {
      if (q <= 0) { this.remove(id); return; }
      if (items[id]) { items[id].qty = q; save(); renderCart(); }
    },
    total()      { return Object.values(items).reduce((s,v) => s + effectivePrice(v.product)*v.qty, 0); },
    count()      { return Object.values(items).reduce((s,v) => s + v.qty, 0); },
    clear()      { items = {}; save(); renderCart(); },
    toOrderItems() { return Object.values(items).map(v => ({ product_id: v.product.id, quantity: v.qty })); },
  };
})();

function effectivePrice(p) {
  return p.discount_pct > 0 ? p.price * (1 - p.discount_pct / 100) : p.price;
}

// ── Init — guests can browse; login required only at checkout ─
(async () => {
  const res = await NS.api('GET', '/api/me');
  if (res.ok) {
    currentUser = res.data;
    NS.setUser(currentUser);
    document.getElementById('navUser').textContent = currentUser.first_name;
    document.getElementById('signinBtn').hidden = true;
    document.getElementById('logoutBtn').hidden = false;
    document.getElementById('logoutBtn').addEventListener('click', NS.logout);
    if (currentUser.role === 'admin') {
      document.getElementById('adminBar').style.display = 'flex';
    }
  } else {
    // Guest — point My Account to login page
    const accountLink = document.getElementById('navAccountLink');
    if (accountLink) accountLink.href = '/login.html';
  }
  renderCart();
  await loadProducts();
})();

// ── Product loading ───────────────────────────────────────────
async function loadProducts(page = 1) {
  currentPage = page;
  const params = new URLSearchParams({ page, size: PAGE_SIZE });
  if (activeCategory) params.set('category', activeCategory);

  const { ok, data } = await NS.api('GET', '/api/products?' + params);
  if (!ok) { NS.toast('Could not load products.', 'error'); return; }

  allProducts   = data.products || [];
  totalProducts = data.total    || 0;
  renderGrid();
  renderFilters(data.products);
  renderPagination();
}

function renderGrid() {
  const grid = document.getElementById('productGrid');
  if (!allProducts.length) {
    grid.innerHTML = '<p style="color:var(--text-soft);font-style:italic;font-size:0.85rem">No products found.</p>';
    return;
  }
  grid.innerHTML = allProducts.map(p => productCard(p)).join('');
  grid.querySelectorAll('.s-card-btn').forEach(btn => {
    btn.addEventListener('click', () => {
      const pid = parseInt(btn.dataset.id);
      const p   = allProducts.find(x => x.id === pid);
      if (p) { cart.add(p); openCart(); NS.toast(p.name + ' added to cart', 'success'); }
    });
  });
  grid.querySelectorAll('.s-card').forEach(card => {
    card.addEventListener('click', (e) => {
      if (e.target.closest('.s-card-btn')) return;
      const pid = parseInt(card.dataset.id);
      const p   = allProducts.find(x => x.id === pid);
      if (p) openProductModal(p);
    });
  });
}

function productCard(p) {
  const ep   = effectivePrice(p);
  const img  = p.image_url
    ? `<img src="${NS.escapeHtml(p.image_url)}" alt="${NS.escapeHtml(p.name)}" loading="lazy"/>`
    : `<div class="s-card-placeholder">No Image</div>`;
  const badge = p.discount_pct > 0 ? `<span class="s-badge-discount">${p.discount_pct}% off</span>` : '';
  const priceHtml = p.discount_pct > 0
    ? `<span class="s-price s-price-sale">${NS.formatCurrency(ep)}</span>
       <span class="s-price-original">${NS.formatCurrency(p.price)}</span>`
    : `<span class="s-price">${NS.formatCurrency(p.price)}</span>`;
  const notes = p.notes?.length ? `<p class="s-card-notes">${NS.escapeHtml(p.notes.join(' · '))}</p>` : '';
  const outOfStock = p.stock <= 0;

  return `
    <article class="s-card" data-id="${p.id}" style="cursor:pointer">
      <div class="s-card-img-wrap">${img}${badge}</div>
      <div class="s-card-body">
        <p class="s-card-category">${NS.escapeHtml(p.category || '')}</p>
        <h3 class="s-card-name">${NS.escapeHtml(p.name)}</h3>
        ${notes}
        <div class="s-card-price-row">${priceHtml}</div>
      </div>
      <div class="s-card-footer">
        <button class="s-card-btn" data-id="${p.id}" ${outOfStock ? 'disabled' : ''}>
          ${outOfStock ? 'Out of Stock' : 'Add to Cart'}
        </button>
      </div>
    </article>`;
}

function renderFilters(products) {
  if (!products) return;
  products.forEach(p => { if (p.category) categories.add(p.category); });
  const bar = document.getElementById('filterBar');
  const existing = bar.querySelectorAll('.s-filter-btn[data-cat]');
  existing.forEach(b => { if (b.dataset.cat !== '') b.remove(); });
  categories.forEach(cat => {
    const btn = document.createElement('button');
    btn.className = 's-filter-btn' + (cat === activeCategory ? ' active' : '');
    btn.dataset.cat = cat;
    btn.textContent = cat;
    bar.appendChild(btn);
  });
  bar.querySelectorAll('.s-filter-btn').forEach(btn => {
    btn.addEventListener('click', () => {
      activeCategory = btn.dataset.cat;
      bar.querySelectorAll('.s-filter-btn').forEach(b => b.classList.remove('active'));
      btn.classList.add('active');
      loadProducts(1);
    });
  });
}

function renderPagination() {
  const wrap   = document.getElementById('pagination');
  const info   = document.getElementById('pageInfo');
  const prev   = document.getElementById('prevPage');
  const next   = document.getElementById('nextPage');
  const pages  = Math.ceil(totalProducts / PAGE_SIZE);
  wrap.hidden  = pages <= 1;
  info.textContent = `Page ${currentPage} of ${pages}`;
  prev.disabled = currentPage <= 1;
  next.disabled = currentPage >= pages;
  prev.onclick = () => loadProducts(currentPage - 1);
  next.onclick = () => loadProducts(currentPage + 1);
}

// ── Product detail modal ──────────────────────────────────────
function openProductModal(p) {
  const ep    = effectivePrice(p);
  const badge = p.discount_pct > 0 ? `<span class="s-badge-discount">${p.discount_pct}% off</span>` : '';
  const priceHtml = p.discount_pct > 0
    ? `<span class="s-price s-price-sale">${NS.formatCurrency(ep)}</span>
       <span class="s-price-original">${NS.formatCurrency(p.price)}</span>`
    : `<span class="s-price">${NS.formatCurrency(p.price)}</span>`;
  const notes = p.notes?.length
    ? `<p style="font-size:0.78rem;color:var(--text-soft);margin-top:0.6rem">${NS.escapeHtml(p.notes.join(' · '))}</p>`
    : '';

  document.getElementById('productModalContent').innerHTML = `
    <button class="s-modal-close" id="productModalClose">&times;</button>
    <div style="display:grid;grid-template-columns:1fr 1fr;gap:2rem;align-items:start">
      <div class="s-card-img-wrap" style="aspect-ratio:3/4;position:relative">
        ${p.image_url
          ? `<img src="${NS.escapeHtml(p.image_url)}" alt="${NS.escapeHtml(p.name)}" style="width:100%;height:100%;object-fit:cover"/>`
          : `<div class="s-card-placeholder">No Image</div>`}
        ${badge}
      </div>
      <div>
        <p style="font-size:0.62rem;letter-spacing:0.2em;text-transform:uppercase;color:var(--text-soft)">${NS.escapeHtml(p.category || '')}</p>
        <h2 style="font-family:var(--font-serif);font-size:1.8rem;font-style:italic;font-weight:400;margin:0.4rem 0;line-height:1.2">${NS.escapeHtml(p.name)}</h2>
        ${notes}
        <p style="font-size:0.82rem;line-height:1.75;color:var(--text-mid);margin:1rem 0">${NS.escapeHtml(p.description || '')}</p>
        <div class="s-card-price-row" style="margin-bottom:1.4rem">${priceHtml}</div>
        <p style="font-size:0.68rem;color:var(--text-soft);margin-bottom:1rem">
          ${p.stock > 0 ? `${p.stock} in stock` : '<span style="color:var(--danger)">Out of stock</span>'}
        </p>
        <button class="s-btn s-btn--gold" id="modalAddCart" data-id="${p.id}" ${p.stock <= 0 ? 'disabled' : ''}>
          Add to Cart
        </button>
      </div>
    </div>
  `;
  document.getElementById('productModal').hidden = false;
  document.getElementById('productModalClose').onclick = () => { document.getElementById('productModal').hidden = true; };
  document.getElementById('modalAddCart')?.addEventListener('click', () => {
    cart.add(p);
    document.getElementById('productModal').hidden = true;
    openCart();
    NS.toast(p.name + ' added to cart', 'success');
  });
}

document.getElementById('productModal').addEventListener('click', e => {
  if (e.target === document.getElementById('productModal')) document.getElementById('productModal').hidden = true;
});

// ── Cart ──────────────────────────────────────────────────────
function openCart()  { document.getElementById('cartDrawer').classList.add('open'); document.getElementById('cartOverlay').classList.add('open'); }
function closeCart() { document.getElementById('cartDrawer').classList.remove('open'); document.getElementById('cartOverlay').classList.remove('open'); }

document.getElementById('cartToggle').addEventListener('click', openCart);
document.getElementById('cartClose').addEventListener('click', closeCart);
document.getElementById('cartOverlay').addEventListener('click', closeCart);

function renderCart() {
  const items = Object.values(cart.items);
  const count = cart.count();
  const badge = document.getElementById('cartCount');
  badge.hidden = count === 0;
  badge.textContent = count;

  const container = document.getElementById('cartItems');
  if (!items.length) {
    container.innerHTML = '<p class="s-cart-empty">Your cart is empty.</p>';
    document.getElementById('cartTotal').textContent = NS.formatCurrency(0);
    return;
  }

  container.innerHTML = items.map(({ product: p, qty }) => `
    <div class="s-cart-item" data-id="${p.id}">
      ${p.image_url
        ? `<img class="s-cart-item-img" src="${NS.escapeHtml(p.image_url)}" alt="${NS.escapeHtml(p.name)}"/>`
        : `<div class="s-cart-item-img" style="background:#f0ede8"></div>`}
      <div class="s-cart-item-info">
        <p class="s-cart-item-name">${NS.escapeHtml(p.name)}</p>
        <p class="s-cart-item-price">${NS.formatCurrency(effectivePrice(p))}</p>
        <div class="s-cart-item-qty">
          <button class="s-qty-btn" data-action="dec" data-id="${p.id}">−</button>
          <span class="s-qty-num">${qty}</span>
          <button class="s-qty-btn" data-action="inc" data-id="${p.id}">+</button>
        </div>
        <button class="s-cart-item-remove" data-id="${p.id}">Remove</button>
      </div>
    </div>
  `).join('');

  document.getElementById('cartTotal').textContent = NS.formatCurrency(cart.total());

  container.querySelectorAll('[data-action]').forEach(btn => {
    btn.addEventListener('click', () => {
      const id  = parseInt(btn.dataset.id);
      const cur = cart.items[id]?.qty || 0;
      if (btn.dataset.action === 'inc') {
        const p = cart.items[id]?.product;
        cart.setQty(id, Math.min(cur + 1, p?.stock || 99));
      } else {
        cart.setQty(id, cur - 1);
      }
    });
  });
  container.querySelectorAll('.s-cart-item-remove').forEach(btn => {
    btn.addEventListener('click', () => cart.remove(parseInt(btn.dataset.id)));
  });
}

// ── Checkout ──────────────────────────────────────────────────
document.getElementById('checkoutBtn').addEventListener('click', () => {
  if (!cart.count()) { NS.toast('Your cart is empty.', 'info'); return; }
  openCheckout();
});

function openCheckout() {
  // Show guest email field only for unauthenticated users
  document.getElementById('guestEmailField').hidden = !!currentUser;
  // Pre-fill from user profile
  if (currentUser) {
    document.getElementById('shipFirst').value = currentUser.first_name || '';
    document.getElementById('shipLast').value  = currentUser.last_name  || '';
  }
  // Summary
  const summaryEl = document.getElementById('checkoutSummary');
  summaryEl.innerHTML = Object.values(cart.items).map(({ product: p, qty }) =>
    `<div style="display:flex;justify-content:space-between">
       <span>${NS.escapeHtml(p.name)} × ${qty}</span>
       <span>${NS.formatCurrency(effectivePrice(p) * qty)}</span>
     </div>`
  ).join('');
  document.getElementById('checkoutTotal').textContent = NS.formatCurrency(cart.total());
  document.getElementById('checkoutModal').hidden = false;
  document.getElementById('checkoutFeedback').textContent = '';
  closeCart();
}

document.getElementById('checkoutClose').addEventListener('click', () => { document.getElementById('checkoutModal').hidden = true; });

// ── Geolocation detect ────────────────────────────────────────
document.getElementById('detectLocationBtn').addEventListener('click', () => {
  const btn = document.getElementById('detectLocationBtn');
  if (!navigator.geolocation) { NS.toast('Geolocation not supported by your browser.', 'info'); return; }
  btn.textContent = '…detecting';
  btn.disabled = true;
  navigator.geolocation.getCurrentPosition(async pos => {
    try {
      const { latitude: lat, longitude: lon } = pos.coords;
      const res = await fetch(
        `https://nominatim.openstreetmap.org/reverse?lat=${lat}&lon=${lon}&format=json`,
        { headers: { 'Accept-Language': 'en' } }
      );
      const place = await res.json();
      const addr  = place.address || {};
      const road    = addr.road || addr.street || '';
      const houseNo = addr.house_number || '';
      document.getElementById('shipLine1').value  = [houseNo, road].filter(Boolean).join(' ');
      document.getElementById('shipCity').value    = addr.city || addr.town || addr.village || '';
      document.getElementById('shipPostal').value  = addr.postcode || '';
      // Try to match country to dropdown
      const countryName = addr.country || '';
      const sel = document.getElementById('shipCountry');
      const opt = [...sel.options].find(o => o.value.toLowerCase() === countryName.toLowerCase());
      if (opt) sel.value = opt.value;
      NS.toast('Location filled in — please verify before placing order.', 'success');
    } catch {
      NS.toast('Could not retrieve address. Please fill in manually.', 'error');
    } finally {
      btn.textContent = '⌖ Detect location';
      btn.disabled = false;
    }
  }, () => {
    NS.toast('Location access denied. Please fill in manually.', 'info');
    btn.textContent = '⌖ Detect location';
    btn.disabled = false;
  });
});
document.getElementById('checkoutModal').addEventListener('click', e => {
  if (e.target === document.getElementById('checkoutModal')) document.getElementById('checkoutModal').hidden = true;
});

document.getElementById('placeOrderBtn').addEventListener('click', async () => {
  const fb  = document.getElementById('checkoutFeedback');
  const btn = document.getElementById('placeOrderBtn');
  fb.className = 's-feedback error';
  fb.textContent = '';

  const first       = document.getElementById('shipFirst').value.trim();
  const last        = document.getElementById('shipLast').value.trim();
  const line1       = document.getElementById('shipLine1').value.trim();
  const city        = document.getElementById('shipCity').value.trim();
  const country     = document.getElementById('shipCountry').value;
  const postal      = document.getElementById('shipPostal').value.trim();
  const payment     = document.getElementById('payMethod').value;
  const guestEmail  = !currentUser ? document.getElementById('guestEmail').value.trim() : '';

  if (!currentUser && !guestEmail) {
    fb.textContent = 'Please enter your email address.'; return;
  }
  if (!first || !last) { fb.textContent = 'Please enter your first and last name.'; return; }
  if (!line1 || !city || !country || !postal) {
    fb.textContent = 'Please fill in all required shipping fields.'; return;
  }
  if (!payment) { fb.textContent = 'Please select a payment method.'; return; }

  NS.btnLoading(btn, true);

  const payload = {
    items:          cart.toOrderItems(),
    payment_method: payment,
    ship_name:    first + ' ' + last,
    ship_line1:   line1,
    ship_line2:   document.getElementById('shipLine2').value.trim(),
    ship_city:    city,
    ship_country: country,
    ship_postal:  postal,
  };
  if (guestEmail) payload.guest_email = guestEmail;

  const { ok, data } = await NS.api('POST', '/api/orders', payload);

  NS.btnLoading(btn, false);

  if (ok) {
    cart.clear();
    document.getElementById('checkoutModal').hidden = true;
    NS.toast('Order placed successfully! Order #' + data.order_id, 'success');
  } else {
    fb.textContent = data.error || 'Could not place order. Please try again.';
  }
});

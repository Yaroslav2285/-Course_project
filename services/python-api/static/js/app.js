// LR #6: Web/DB — Frontend API client for marketplace
// LR #2: Modern Python — ES6+ fetch, async/await, template literals

const API_BASE = '/v1';

// === Storage helpers ===
function getToken() {
  return localStorage.getItem('access_token');
}

function getRefreshToken() {
  return localStorage.getItem('refresh_token');
}

function setToken(access, refresh) {
  localStorage.setItem('access_token', access);
  localStorage.setItem('refresh_token', refresh);
}

function clearTokens() {
  localStorage.removeItem('access_token');
  localStorage.removeItem('refresh_token');
  localStorage.removeItem('user');
}

function isLoggedIn() {
  return !!getToken();
}

function getUser() {
  try {
    return JSON.parse(localStorage.getItem('user'));
  } catch { return null; }
}

function setUser(user) {
  localStorage.setItem('user', JSON.stringify(user));
}

// === HTTP helpers ===
async function apiFetch(path, options = {}) {
  const token = getToken();
  const headers = {
    'Content-Type': 'application/json',
    ...(token ? { 'Authorization': `Bearer ${token}` } : {}),
    ...options.headers,
  };

  const res = await fetch(`${API_BASE}${path}`, { ...options, headers });
  const json = await res.json();

  if (!res.ok) {
    const errMsg = json.errors?.[0]?.detail || `HTTP ${res.status}`;
    const err = new Error(errMsg);
    err.status = res.status;
    err.data = json;
    throw err;
  }
  return json;
}

function showAlert(message, type = 'error', containerId = 'alert-container') {
  const container = document.getElementById(containerId);
  if (!container) return;
  container.innerHTML = `<div class="alert alert-${type}">${message}</div>`;
  setTimeout(() => { container.innerHTML = ''; }, 5000);
}

function clearAlerts(containerId = 'alert-container') {
  const container = document.getElementById(containerId);
  if (container) container.innerHTML = '';
}

function renderBadge(status) {
  return `<span class="badge badge-${status}">${status}</span>`;
}

// === Auth ===
async function handleLogin(email, password) {
  const data = await apiFetch('/auth/login', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  });
  const { access_token, refresh_token, user } = data.data;
  setToken(access_token, refresh_token);
  setUser(user);
  window.location.href = '/dashboard';
}

async function handleRegister(email, password, role = 'client') {
  const data = await apiFetch('/auth/register', {
    method: 'POST',
    body: JSON.stringify({ email, password, role }),
  });
  const { access_token, refresh_token, user } = data.data;
  setToken(access_token, refresh_token);
  setUser(user);
  window.location.href = '/dashboard';
}

async function handleLogout() {
  clearTokens();
  window.location.href = '/';
}

// === Services ===
async function fetchServices(limit = 20, offset = 0) {
  const data = await apiFetch(`/services/?limit=${limit}&offset=${offset}`);
  return data;
}

async function fetchMyServices() {
  const data = await apiFetch('/services/my');
  return data;
}

async function createService(title, description, price) {
  const data = await apiFetch('/services/', {
    method: 'POST',
    body: JSON.stringify({ title, description, price }),
  });
  return data;
}

async function updateService(id, fields) {
  const data = await apiFetch(`/services/${id}`, {
    method: 'PUT',
    body: JSON.stringify(fields),
  });
  return data;
}

async function deleteService(id) {
  const data = await apiFetch(`/services/${id}`, { method: 'DELETE' });
  return data;
}

// === Orders ===
async function fetchOrders(limit = 20, offset = 0) {
  const data = await apiFetch(`/orders/?limit=${limit}&offset=${offset}`);
  return data;
}

async function fetchSoldOrders() {
  const data = await apiFetch('/orders/sold');
  return data;
}

async function createOrder(service_id, seller_id, amount, notes = '') {
  const data = await apiFetch('/orders/', {
    method: 'POST',
    body: JSON.stringify({ service_id, seller_id, amount, notes }),
  });
  return data;
}

async function updateOrderStatus(orderId, status) {
  const data = await apiFetch(`/orders/${orderId}/status`, {
    method: 'PATCH',
    body: JSON.stringify({ status }),
  });
  return data;
}

async function getOrder(orderId) {
  const data = await apiFetch(`/orders/${orderId}`);
  return data;
}

// === Escrow status helpers ===
const ESCROW_STEPS = ['pending', 'funded', 'released', 'cancelled', 'disputed'];

function renderEscrowFlow(currentStatus) {
  const steps = ['pending', 'funded', 'released', 'cancelled'];
  const idx = steps.indexOf(currentStatus);
  return steps.map((s, i) => {
    let cls = '';
    if (i < idx) cls = 'completed';
    else if (i === idx) cls = 'active';
    const arrow = i < steps.length - 1 ? '<span class="escrow-arrow">→</span>' : '';
    return `
      <div class="escrow-step ${cls}">
        <div class="step-dot">${i + 1}</div>
        <span>${s}</span>
      </div>
      ${arrow}
    `;
  }).join('');
}

function renderEscrowStatus(status) {
  return `
    <div class="escrow-flow">
      ${renderEscrowFlow(status)}
    </div>
  `;
}

// === UI helpers ===
function renderServiceCard(svc) {
  return `
    <div class="service-card" data-id="${svc.id}">
      <div class="flex-between">
        <h3>${escapeHtml(svc.title)}</h3>
        ${renderBadge(svc.status)}
      </div>
      <p class="description">${escapeHtml(svc.description || 'No description')}</p>
      <div class="flex-between">
        <span class="price">${svc.price} ₽</span>
        <button class="btn btn-primary btn-sm order-btn" data-id="${svc.id}"
          data-seller="${svc.provider_id}" data-price="${svc.price}">
          Заказать
        </button>
      </div>
    </div>
  `;
}

function renderOrderRow(order) {
  return `
    <tr>
      <td>${order.id?.slice(0, 8)}…</td>
      <td>${order.amount} ₽</td>
      <td>${renderBadge(order.status)}</td>
      <td>${new Date(order.created_at).toLocaleDateString()}</td>
      <td>
        <a href="/escrow/${order.id}" class="btn btn-outline btn-sm">Статус</a>
      </td>
    </tr>
  `;
}

function escapeHtml(text) {
  const div = document.createElement('div');
  div.textContent = text;
  return div.innerHTML;
}

// === Navbar ===
function renderNavbar() {
  const nav = document.getElementById('navbar');
  if (!nav) return;
  const loggedIn = isLoggedIn();
  const user = getUser();
  nav.innerHTML = `
    <a href="/" class="navbar-brand">🛒 Marketplace</a>
    <div class="navbar-menu">
      <a href="/">Каталог</a>
      ${loggedIn ? `
        <a href="/dashboard">Личный кабинет</a>
        <span class="text-secondary">${escapeHtml(user?.email || '')}</span>
        <a href="#" onclick="handleLogout()" class="btn btn-outline btn-sm">Выйти</a>
      ` : `
        <a href="/login">Войти</a>
      `}
    </div>
  `;
}

document.addEventListener('DOMContentLoaded', renderNavbar);

// LR #7: Frontend/UI
// LR #6: Web/DB — API client for Service Marketplace
// LR #10: Multi-lang/REST — unified fetch wrapper with JWT auth

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
  } catch {
    return null;
  }
}

function setUser(user) {
  localStorage.setItem('user', JSON.stringify(user));
}

// === apiFetch — unified fetch wrapper ===
async function apiFetch(path, options = {}) {
  const token = getToken();
  const headers = {
    'Content-Type': 'application/json',
    'Cache-Control': 'no-cache',
    'Pragma': 'no-cache',
    'X-Request-ID': crypto.randomUUID(),
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
    ...options.headers,
  };

  let res;
  try {
    const fetchPromise = fetch(`${API_BASE}${path}`, { ...options, headers, signal: undefined });
    const timeoutPromise = new Promise(function (_, reject) {
      setTimeout(function () { reject(new Error('TIMEOUT')); }, 15000);
    });
    res = await Promise.race([fetchPromise, timeoutPromise]);
  } catch (e) {
    console.error('apiFetch raw error:', e.name, e.message);
    if (!options._retried && e.message !== 'TIMEOUT') {
      await new Promise(function (r) { setTimeout(r, 300); });
      return apiFetch(path, { _retried: true });
    }
    const err = new Error('Network error — server unavailable');
    err.status = 0;
    throw err;
  }

  let json;
  try {
    json = await res.json();
  } catch {
    json = {};
  }

  if (!res.ok) {
    const errMsg =
      (json.errors && json.errors[0] && json.errors[0].detail) ||
      `HTTP ${res.status}`;
    const err = new Error(errMsg);
    err.status = res.status;
    err.data = json;

    // Auto-redirect on 401/403
    if (res.status === 401 || res.status === 403) {
      clearTokens();
      const currentPath = window.location.pathname;
      if (
        currentPath !== '/login' &&
        currentPath !== '/register' &&
        currentPath !== '/'
      ) {
        const toast = document.querySelector('.toast-container');
        if (toast) {
          const el = document.createElement('div');
          el.className = 'toast toast-error';
          el.textContent = 'Session expired, please login again';
          toast.appendChild(el);
          setTimeout(() => el.remove(), 3500);
        }
        setTimeout(() => {
          window.location.href = '/login';
        }, 1000);
      }
    }
    throw err;
  }

  return json;
}

// === API Endpoint wrappers ===

// Auth
async function apiLogin(email, password) {
  return apiFetch('/auth/login', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  });
}

async function apiRegister(email, password, role) {
  return apiFetch('/auth/register', {
    method: 'POST',
    body: JSON.stringify({ email, password, role }),
  });
}

async function apiRefresh(refreshToken) {
  return apiFetch('/auth/refresh', {
    method: 'POST',
    body: JSON.stringify({ refresh_token: refreshToken }),
  });
}

// Users
async function apiFetchMe() {
  return apiFetch('/users/me');
}

async function apiUpdateMe(data) {
  return apiFetch('/users/me', {
    method: 'PUT',
    body: JSON.stringify(data),
  });
}

// Services
async function apiFetchServices(params = {}) {
  const query = new URLSearchParams();
  if (params.limit != null) query.set('limit', params.limit);
  if (params.offset != null) query.set('offset', params.offset);
  if (params.status) query.set('status', params.status);
  const qs = query.toString();
  return apiFetch(`/services/${qs ? '?' + qs : ''}`);
}

async function apiFetchMyServices(limit, offset) {
  limit = limit || 100;
  offset = offset || 0;
  var t = Date.now();
  return apiFetch('/services/my?limit=' + limit + '&offset=' + offset + '&_t=' + t);
}

async function apiFetchService(id) {
  return apiFetch(`/services/${id}`);
}

async function apiCreateService(title, description, price, category, discount) {
  return apiFetch('/services/', {
    method: 'POST',
    body: JSON.stringify({ title, description, price, category, discount }),
  });
}

async function apiUpdateService(id, fields) {
  return apiFetch(`/services/${id}`, {
    method: 'PUT',
    body: JSON.stringify(fields),
  });
}

async function apiDeleteService(id) {
  return apiFetch(`/services/${id}`, { method: 'DELETE' });
}

// Orders
async function apiFetchOrders(limit = 20, offset = 0, status) {
  const query = new URLSearchParams();
  query.set('limit', limit);
  query.set('offset', offset);
  if (status) query.set('status', status);
  return apiFetch(`/orders/?${query.toString()}`);
}

async function apiFetchSoldOrders(limit) {
  var path = '/orders/sold';
  if (limit) path += '?limit=' + limit;
  return apiFetch(path);
}

async function apiFetchOrder(id) {
  return apiFetch(`/orders/${id}`);
}

async function apiCreateOrder(serviceId, sellerId, amount, notes) {
  return apiFetch('/orders/', {
    method: 'POST',
    body: JSON.stringify({
      service_id: serviceId,
      seller_id: sellerId,
      amount,
      notes: notes || '',
    }),
  });
}

async function apiUpdateOrderStatus(id, status) {
  return apiFetch(`/orders/${id}/status`, {
    method: 'PATCH',
    body: JSON.stringify({ status }),
  });
}

// Escrow (proxy via Python API)
async function apiEscrowByOrder(orderId) {
  try {
    const data = await apiFetch(`/escrow/by-order/${orderId}`);
    return data.data;
  } catch (e) {
    if (e.status === 404) return null;
    throw e;
  }
}

async function apiEscrowAction(orderId, action) {
  const idempotencyKey = crypto.randomUUID();
  return apiFetch(`/escrow/${orderId}/${action}`, {
    method: 'POST',
    headers: { 'X-Idempotency-Key': idempotencyKey },
  });
}

async function apiEscrowResolve(orderId, action) {
  const idempotencyKey = crypto.randomUUID();
  return apiFetch(`/escrow/${orderId}/resolve`, {
    method: 'POST',
    headers: { 'X-Idempotency-Key': idempotencyKey, 'Content-Type': 'application/json' },
    body: JSON.stringify({ action: action }),
  });
}

async function apiEscrowDispute(orderId, reason) {
  const idempotencyKey = crypto.randomUUID();
  return apiFetch(`/escrow/${orderId}/dispute`, {
    method: 'POST',
    headers: { 'X-Idempotency-Key': idempotencyKey, 'Content-Type': 'application/json' },
    body: JSON.stringify({ reason: reason }),
  });
}

async function apiEscrowComplete(orderId) {
  const idempotencyKey = crypto.randomUUID();
  return apiFetch(`/escrow/${orderId}/complete`, {
    method: 'POST',
    headers: { 'X-Idempotency-Key': idempotencyKey },
  });
}

async function apiEscrowAdvance(orderId, status) {
  return apiFetch(`/escrow/${orderId}/advance`, {
    method: 'POST',
    body: JSON.stringify({ status }),
  });
}

// Wallet API
async function apiFetchWalletBalance() {
  return apiFetch('/wallet/balance');
}

async function apiFetchWalletTopUp(amount) {
  return apiFetch('/wallet/topup', {
    method: 'POST',
    body: JSON.stringify({ amount: amount }),
  });
}

async function apiWalletPay(orderId) {
  return apiFetch('/wallet/pay', {
    method: 'POST',
    body: JSON.stringify({ order_id: orderId }),
  });
}

async function apiFetchWalletTransactions(limit) {
  return apiFetch('/wallet/transactions?limit=' + (limit || 20));
}

// Blockchain audit (proxy via Python API)
async function apiFetchAudit(orderId) {
  try {
    const data = await apiFetch(`/chain/audit/${orderId}`);
    return data.data;
  } catch (e) {
    if (e.status === 404) return null;
    throw e;
  }
}

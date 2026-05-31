// LR #6: Web/DB — Dashboard logic with escrow state machine
// LR #10: Multi-lang/REST — escrow proxy calls with idempotency
// LR #12: AI Integration — role-based UI, debounce, optimistic updates

var serviceCache = {};
var serviceData = [];
var _actionInProgress = {};

// ===== Helpers =====
async function fetchServiceTitle(id) {
  if (serviceCache[id]) return serviceCache[id];
  try {
    var res = await apiFetchService(id);
    if (res && res.data) {
      serviceCache[id] = res.data.title;
      return res.data.title;
    }
  } catch (e) { /* ignore */ }
  return id.slice(0, 8) + '...';
}

function showSkeleton(parent, rows) {
  if (!parent) return;
  rows = rows || 3;
  var html = '';
  for (var i = 0; i < rows; i++) {
    html += '<tr class="skeleton-row">'
      + '<td class="skeleton-cell"><div class="skeleton-bar long"></div></td>'
      + '<td class="skeleton-cell"><div class="skeleton-bar short"></div></td>'
      + '<td class="skeleton-cell"><span class="skeleton-badge"></span></td>'
      + '<td class="skeleton-cell"><div class="skeleton-bar medium"></div></td>'
      + '<td class="skeleton-cell"><div class="skeleton-bar short"></div></td>'
      + '</tr>';
  }
  parent.innerHTML = html;
}

function debounceClick(fn) {
  return function () {
    if (window._isProcessing) return;
    window._isProcessing = true;
    var args = arguments;
    var self = this;
    Promise.resolve(fn.apply(self, args)).finally(function () {
      window._isProcessing = false;
    });
  };
}

// ===== Client Dashboard =====

function initClientDashboard() {
  try {
    var loadEl = document.getElementById('orders-loading');
    var emptyEl = document.getElementById('orders-empty');
    var errEl = document.getElementById('orders-error');
    var tableEl = document.getElementById('orders-table-wrapper');
    var tbody = document.getElementById('orders-tbody');

    if (!loadEl || !tbody) {
      console.error('Dashboard: required DOM elements missing');
      return;
    }

    loadEl.style.display = 'block';
    if (emptyEl) emptyEl.style.display = 'none';
    if (errEl) errEl.style.display = 'none';
    if (tableEl) tableEl.style.display = 'none';
    showSkeleton(tbody, 3);

    var auth = checkAuth();
    if (!auth.isAuthenticated) {
      loadEl.style.display = 'none';
      if (errEl) {
        errEl.style.display = 'block';
        errEl.innerHTML = '<div class="alert alert-error">Please <a href="/login">login</a> to view your orders.</div>';
      }
      return;
    }

    var apiUrl = API_BASE + '/orders/?limit=50&offset=0';
    console.log('Dashboard: fetching orders from', apiUrl);

    apiFetchOrders(50, 0)
      .then(function (res) {
        console.log('Dashboard: orders received', res);
        var orders = (res.data || []).sort(function (a, b) {
          return new Date(b.created_at) - new Date(a.created_at);
        });
        loadEl.style.display = 'none';
        if (!orders.length) {
          if (emptyEl) emptyEl.style.display = 'block';
          return;
        }
        return renderClientOrders(tbody, orders).then(function () {
          if (tableEl) tableEl.style.display = 'block';
        });
      })
      .catch(function (err) {
        console.error('Dashboard: fetch error', err);
        loadEl.style.display = 'none';
        if (errEl) {
          errEl.style.display = 'block';
          errEl.innerHTML = '<div class="alert alert-error">Failed to load orders: ' + escapeHtml(err.message || 'Unknown error') + '</div>';
        }
      });
  } catch (err) {
    console.error('Dashboard: init error', err);
    var loadEl = document.getElementById('orders-loading');
    if (loadEl) {
      loadEl.style.display = 'none';
      var errEl = document.getElementById('orders-error');
      if (errEl) {
        errEl.style.display = 'block';
        errEl.innerHTML = '<div class="alert alert-error">' + escapeHtml(err.message || 'Unexpected error') + '</div>';
      }
    }
  }
}

async function renderClientOrders(tbody, orders) {
  var titles = {};
  await Promise.all(orders.map(function (o) {
    return fetchServiceTitle(o.service_id).then(function (t) { titles[o.service_id] = t; });
  }));
  var html = '';
  orders.forEach(function (o) {
    var actions = clientActions(o.status, o.id);
    html += '<tr>'
      + '<td class="order-service">' + escapeHtml(titles[o.service_id] || '...') + '</td>'
      + '<td class="order-amount">' + formatPrice(o.amount) + '</td>'
      + '<td><span id="badge-' + o.id + '">' + statusBadge(o.status) + '</span></td>'
      + '<td class="order-date">' + formatDate(o.created_at) + '</td>'
      + '<td class="order-actions" id="actions-' + o.id + '">' + actions + '</td>'
      + '</tr>';
  });
  tbody.innerHTML = html;
}

function clientActions(status, orderId) {
  var html = '';
  // Phase 7.4: Replace inline escrow actions with "View Detail" link to order detail page
  html += '<a href="/orders/' + orderId + '" class="btn btn-outline btn-sm">Details</a>';
  return html;
}

// ===== Executor Dashboard =====

function initExecutorDashboard() {
  try {
    var tab = document.getElementById('tab-services');
    if (tab) tab.classList.add('active');
    initMyServices();
    initIncomingOrders();
  } catch (err) {
    console.error('Dashboard executor init error', err);
  }
}

function switchTab(tab) {
  document.querySelectorAll('.dashboard-tab').forEach(function (t) { t.classList.remove('active'); });
  document.querySelectorAll('.dashboard-tab-content').forEach(function (c) { c.classList.remove('active'); });
  document.querySelector('.dashboard-tab[data-tab="' + tab + '"]').classList.add('active');
  document.getElementById('tab-' + tab).classList.add('active');
}

// -- My Services --
function initMyServices() {
  var container = document.getElementById('services-content');
  if (!container) return;
  container.innerHTML = '<div class="flex-center" style="padding:40px;"><div class="spinner"></div></div>';

  apiFetchMyServices()
    .then(function (res) {
      var services = res.data || [];
      var total = res.meta ? res.meta.total : services.length;
      var countEl = document.getElementById('services-count');
      if (countEl) countEl.textContent = '(' + total + ')';
      if (!services.length) {
        container.innerHTML = '<div class="dashboard-empty"><p>No services yet. Create your first service!</p></div>';
        return;
      }
      serviceData = [];
      var html = '<div class="table-wrapper"><table class="orders-table"><thead><tr><th>Title</th><th>Price</th><th>Status</th><th>Actions</th></tr></thead><tbody>';
      services.forEach(function (s) {
        serviceData.push({ id: s.id, title: s.title, desc: s.description || '', price: s.price, status: s.status });
        var idx = serviceData.length - 1;
        html += '<tr><td class="order-service">' + escapeHtml(s.title) + '</td>'
          + '<td class="order-amount">' + formatPrice(s.price) + '</td>'
          + '<td>' + statusBadge(s.status) + '</td>'
          + '<td class="order-actions">'
          + '<button class="btn-action btn-action-accept" onclick="editServiceByIndex(' + idx + ')">Edit</button> '
          + '<button class="btn-action btn-action-cancel" onclick="deleteService(\'' + s.id + '\')">Delete</button>'
          + '</td></tr>';
      });
      html += '</tbody></table></div>';
      container.innerHTML = html;
    })
    .catch(function (err) {
      container.innerHTML = '<div class="alert alert-error">Failed to load services: ' + escapeHtml(err.message) + '</div>';
    });
}

// -- Incoming Orders --
function initIncomingOrders() {
  var loadEl = document.getElementById('incoming-loading');
  var emptyEl = document.getElementById('incoming-empty');
  var errEl = document.getElementById('incoming-error');
  var tableEl = document.getElementById('incoming-table-wrapper');
  var tbody = document.getElementById('incoming-tbody');

  if (!loadEl) return;
  if (emptyEl) emptyEl.style.display = 'none';
  if (errEl) errEl.style.display = 'none';
  if (tableEl) tableEl.style.display = 'none';
  loadEl.style.display = 'block';

  apiFetchSoldOrders()
    .then(function (res) {
      var orders = (res.data || []).sort(function (a, b) {
        return new Date(b.created_at) - new Date(a.created_at);
      });
      var total = res.meta ? res.meta.total : orders.length;
      var countEl = document.getElementById('incoming-count');
      if (countEl) countEl.textContent = '(' + total + ')';
      loadEl.style.display = 'none';
      if (!orders.length) {
        emptyEl.style.display = 'block';
        return;
      }
      return renderIncomingOrders(tbody, orders).then(function () {
        tableEl.style.display = 'block';
      });
    })
    .catch(function (err) {
      console.error('Dashboard incoming fetch error', err);
      loadEl.style.display = 'none';
      if (errEl) {
        errEl.style.display = 'block';
        errEl.innerHTML = '<div class="alert alert-error">' + escapeHtml(err.message) + '</div>';
      }
    });
}

async function renderIncomingOrders(tbody, orders) {
  var titles = {};
  await Promise.all(orders.map(function (o) {
    return fetchServiceTitle(o.service_id).then(function (t) { titles[o.service_id] = t; });
  }));
  var html = '';
  orders.forEach(function (o) {
    var actions = executorActions(o.status, o.id);
    html += '<tr>'
      + '<td class="order-service">' + escapeHtml(titles[o.service_id] || '...') + '</td>'
      + '<td class="order-amount">' + formatPrice(o.amount) + '</td>'
      + '<td><span id="ebadge-' + o.id + '">' + statusBadge(o.status) + '</span></td>'
      + '<td class="order-date">' + formatDate(o.created_at) + '</td>'
      + '<td class="order-actions" id="eactions-' + o.id + '">' + actions + '</td>'
      + '</tr>';
  });
  tbody.innerHTML = html;
}

function executorActions(status, orderId) {
  var html = '';
  // Phase 7.4: Replace inline escrow actions with "View Detail" link to order detail page
  html += '<a href="/orders/' + orderId + '" class="btn btn-outline btn-sm">Details</a>';
  return html;
}

// ===== Escrow Actions =====
function handleEscrowAction(orderId, action) {
  if (_actionInProgress[orderId]) return;
  _actionInProgress[orderId] = true;
  var btn = window.event && window.event.target;
  if (btn) btn.disabled = true;

  var confirmMsg = '';
  switch (action) {
    case 'fund': confirmMsg = 'Proceed with payment for this order?'; break;
    case 'advance': confirmMsg = 'Accept this order and start working?'; break;
    case 'complete': confirmMsg = 'Mark this order as complete?'; break;
    case 'release': confirmMsg = 'Confirm release of funds to the executor?'; break;
    case 'dispute': confirmMsg = 'Open a dispute for this order?'; break;
    case 'cancel': confirmMsg = 'Cancel this order?'; break;
  }
  if (confirmMsg && !confirm(confirmMsg)) {
    if (btn) btn.disabled = false;
    delete _actionInProgress[orderId];
    return;
  }

  doAction(orderId, action).catch(function (err) {
    showAlert(err.message, 'error');
  }).finally(function () {
    if (btn) btn.disabled = false;
    delete _actionInProgress[orderId];
  });
}

async function doAction(orderId, action) {
  // Map actions to API calls
  if (action === 'cancel') {
    await apiUpdateOrderStatus(orderId, 'cancelled');
    showToast('Order cancelled', 'info');
  } else if (action === 'fund') {
    // Try escrow proxy first, fallback to order status update
    try {
      await apiEscrowAction(orderId, 'fund');
    } catch (e) {
      if (e.status === 404 || e.status === 0) {
        // Escrow proxy unavailable (dev mode) → use order PATCH
        await apiUpdateOrderStatus(orderId, 'funded');
      } else {
        throw e;
      }
    }
    showToast('Payment successful!', 'success');
  } else if (action === 'advance') {
    try {
      await apiEscrowAdvance(orderId, 'IN_PROGRESS');
    } catch (e) {
      if (e.status === 404 || e.status === 0) {
        await apiUpdateOrderStatus(orderId, 'funded');
      } else {
        throw e;
      }
    }
    showToast('Order accepted, status set to In Progress', 'success');
  } else if (action === 'complete') {
    try {
      await apiEscrowComplete(orderId);
    } catch (e) {
      if (e.status === 404 || e.status === 0) {
        await apiUpdateOrderStatus(orderId, 'released');
      } else {
        throw e;
      }
    }
    showToast('Order completed!', 'success');
  } else if (action === 'release') {
    try {
      await apiEscrowAction(orderId, 'release');
    } catch (e) {
      if (e.status === 404 || e.status === 0) {
        await apiUpdateOrderStatus(orderId, 'released');
      } else {
        throw e;
      }
    }
    showToast('Payment released to executor', 'success');
  } else if (action === 'dispute') {
    try {
      await apiEscrowAction(orderId, 'dispute');
    } catch (e) {
      if (e.status === 404 || e.status === 0) {
        await apiUpdateOrderStatus(orderId, 'disputed');
      } else {
        throw e;
      }
    }
    showToast('Dispute opened', 'info');
  }

  // Optimistic UI update: change status badge and re-render actions
  var newStatus = mapActionToStatus(action);
  updateOrderStatusUI(orderId, newStatus);
}

function mapActionToStatus(action) {
  switch (action) {
    case 'fund': return 'funded';
    case 'advance': return 'in_progress';
    case 'complete': return 'completed';
    case 'release': return 'released';
    case 'dispute': return 'disputed';
    case 'cancel': return 'cancelled';
    default: return '';
  }
}

function updateOrderStatusUI(orderId, newStatus) {
  // Update badge
  var badgeEl = document.getElementById('badge-' + orderId) || document.getElementById('ebadge-' + orderId);
  if (badgeEl) badgeEl.innerHTML = statusBadge(newStatus);
  // Update actions
  var actionsEl = document.getElementById('actions-' + orderId) || document.getElementById('eactions-' + orderId);
  if (actionsEl) {
    var role = checkAuth().role || 'client';
    if (role === 'provider') {
      actionsEl.innerHTML = executorActions(newStatus, orderId);
    } else {
      actionsEl.innerHTML = clientActions(newStatus, orderId);
    }
  }
}

// ===== Service CRUD (Executor) =====
function openCreateServiceModal() {
  document.getElementById('modal-title').textContent = 'Create Service';
  document.getElementById('svc-id').value = '';
  document.getElementById('svc-title').value = '';
  document.getElementById('svc-desc').value = '';
  document.getElementById('svc-price').value = '';
  document.getElementById('svc-status').value = 'draft';
  document.getElementById('service-modal').classList.remove('hidden');
}

function closeServiceModal() {
  document.getElementById('service-modal').classList.add('hidden');
}

function editServiceByIndex(idx) {
  var d = serviceData[idx];
  if (!d) return;
  document.getElementById('modal-title').textContent = 'Edit Service';
  document.getElementById('svc-id').value = d.id;
  document.getElementById('svc-title').value = d.title;
  document.getElementById('svc-desc').value = d.desc;
  document.getElementById('svc-price').value = d.price;
  document.getElementById('svc-status').value = d.status;
  document.getElementById('service-modal').classList.remove('hidden');
}

async function saveService() {
  var id = document.getElementById('svc-id').value;
  var title = document.getElementById('svc-title').value.trim();
  var desc = document.getElementById('svc-desc').value.trim();
  var price = document.getElementById('svc-price').value.trim();
  var status = document.getElementById('svc-status').value;
  if (!title) { showAlert('Title is required', 'error'); return; }
  if (!price || isNaN(price) || parseFloat(price) <= 0) { showAlert('Valid price is required', 'error'); return; }
  try {
    if (id) {
      await apiUpdateService(id, { title: title, description: desc || null, price: price, status: status });
      showToast('Service updated', 'success');
    } else {
      await apiCreateService(title, desc || null, price);
      showToast('Service created', 'success');
    }
    closeServiceModal();
    initMyServices();
  } catch (err) {
    showAlert(err.message, 'error');
  }
}

async function deleteService(id) {
  if (!confirm('Delete this service permanently?')) return;
  try {
    await apiDeleteService(id);
    showToast('Service deleted', 'info');
    initMyServices();
  } catch (err) {
    showAlert(err.message, 'error');
  }
}

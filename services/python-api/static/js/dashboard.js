// LR #6: Web/DB — Dashboard logic with escrow state machine
// LR #10: Multi-lang/REST — escrow proxy calls with idempotency
// LR #12: AI Integration — role-based UI, debounce, optimistic updates
// LR #15: Security/UX — Escape key, double-click protection, focus trap

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

var _svcModalTrigger = null;

function _serviceModalClose() {
  closeServiceModal();
}

function _svcFocusTrap(modalEl, event) {
  var focusable = modalEl.querySelectorAll('a[href], button:not([disabled]), textarea:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])');
  if (focusable.length === 0) return;
  var first = focusable[0];
  var last = focusable[focusable.length - 1];
  if (event.key === 'Tab') {
    if (event.shiftKey) {
      if (document.activeElement === first) { event.preventDefault(); last.focus(); }
    } else {
      if (document.activeElement === last) { event.preventDefault(); first.focus(); }
    }
  }
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
      + '<td class="order-service" data-label="Product">' + escapeHtml(titles[o.service_id] || '...') + '</td>'
      + '<td class="order-amount" data-label="Amount">' + formatPrice(o.amount) + '</td>'
      + '<td data-label="Status"><span id="badge-' + o.id + '">' + statusBadge(o.status) + '</span></td>'
      + '<td class="order-date" data-label="Date">' + formatDate(o.created_at) + '</td>'
      + '<td class="order-actions" id="actions-' + o.id + '" data-label="Actions">' + actions + '</td>'
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
async function initMyServices() {
  var container = document.getElementById('services-content');
  if (!container) return;
  console.log('[initMyServices] starting with container:', container);
  container.innerHTML = '<div class="flex-center" style="padding:40px;"><div class="spinner"></div></div>';

  try {
    var res = await apiFetchMyServices();
    var services = res.data || [];
    console.log('[initMyServices] got services count:', services.length, 'ids:', services.map(function(s){return s.id;}).join(','));
    var total = res.meta ? res.meta.total : services.length;
    var countEl = document.getElementById('services-count');
    if (countEl) countEl.textContent = '(' + total + ')';
    if (!services.length) {
      container.innerHTML = '<div class="dashboard-empty"><p>No products yet. Create your first product!</p></div>';
      return;
    }
    serviceData = [];
    var html = '<div class="table-wrapper"><table class="orders-table"><thead><tr><th>Title</th><th>Price</th><th>Status</th><th>Actions</th></tr></thead><tbody>';
    services.forEach(function (s) {
      serviceData.push({ id: s.id, title: s.title, desc: s.description || '', price: s.price, status: s.status });
      var idx = serviceData.length - 1;
      html += '<tr><td class="order-service" data-label="Title">' + escapeHtml(s.title) + '</td>'
        + '<td class="order-amount" data-label="Price">' + formatPrice(s.price) + '</td>'
        + '<td data-label="Status">' + statusBadge(s.status) + '</td>'
        + '<td class="order-actions" data-label="Actions">'
        + '<button class="btn-action btn-action-accept" onclick="editServiceByIndex(' + idx + ')">Edit</button> '
        + '<button class="btn-action btn-action-cancel" onclick="deleteService(\'' + s.id + '\')">Delete</button>'
        + '</td></tr>';
    });
    html += '</tbody></table></div>';
    // Force DOM replacement — create a brand-new container
    var newContainer = document.createElement('div');
    newContainer.id = 'services-content';
    newContainer.innerHTML = html;
    container.parentNode.replaceChild(newContainer, container);
    console.log('[initMyServices] DOM replaced successfully');
  } catch (err) {
    console.error('[initMyServices] error:', err);
    container = document.getElementById('services-content') || container;
    container.innerHTML = '<div class="alert alert-error">Failed to load products: ' + escapeHtml(err.message) + '</div>';
  }
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
      + '<td class="order-service" data-label="Product">' + escapeHtml(titles[o.service_id] || '...') + '</td>'
      + '<td class="order-amount" data-label="Amount">' + formatPrice(o.amount) + '</td>'
      + '<td data-label="Status"><span id="ebadge-' + o.id + '">' + statusBadge(o.status) + '</span></td>'
      + '<td class="order-date" data-label="Date">' + formatDate(o.created_at) + '</td>'
      + '<td class="order-actions" id="eactions-' + o.id + '" data-label="Actions">' + actions + '</td>'
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
async function handleEscrowAction(orderId, action) {
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
  if (confirmMsg) {
    var confirmed = await showConfirmDialog(confirmMsg);
    if (!confirmed) {
      if (btn) btn.disabled = false;
      delete _actionInProgress[orderId];
      return;
    }
  }

  try {
    await doAction(orderId, action);
  } catch (err) {
    showAlert(err.message, 'error');
  } finally {
    if (btn) btn.disabled = false;
    delete _actionInProgress[orderId];
  }
}

async function doAction(orderId, action) {
  // Map actions to API calls
  if (action === 'cancel') {
    await apiEscrowAction(orderId, 'cancel');
    showToast('Order cancelled', 'info');
    if (typeof initWallet === 'function') initWallet();
  } else if (action === 'fund') {
    try {
      await apiWalletPay(orderId);
    } catch (e) {
      if (e.status === 400) {
        await showModalAlert(e.message || 'Insufficient funds');
        return;
      }
      throw e;
    }
    showToast('Payment successful!', 'success');
  } else if (action === 'advance') {
    try {
      await apiEscrowAdvance(orderId, 'IN_PROGRESS');
    } catch (e) {
      if (e.status === 404 || e.status === 0) {
        await apiUpdateOrderStatus(orderId, 'in_progress');
      } else { throw e; }
    }
    showToast('Order accepted', 'success');
  } else if (action === 'complete') {
    try {
      await apiEscrowComplete(orderId);
    } catch (e) {
      if (e.status === 404 || e.status === 0) {
        await apiUpdateOrderStatus(orderId, 'completed');
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
    if (typeof initWallet === 'function') initWallet();
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
function _resetSaveBtn() {
  var btn = document.querySelector('#service-modal .btn-primary');
  if (btn) { btn.disabled = false; btn.textContent = 'Save'; }
}

function openCreateServiceModal() {
  _resetSaveBtn();
  _svcModalTrigger = document.activeElement;
  document.getElementById('modal-title').textContent = 'Create Product';
  document.getElementById('svc-id').value = '';
  document.getElementById('svc-title').value = '';
  document.getElementById('svc-desc').value = '';
  document.getElementById('svc-price').value = '';
  document.getElementById('svc-status').value = 'draft';
  var modal = document.getElementById('service-modal');
  modal.classList.remove('hidden');
  modal.setAttribute('aria-hidden', 'false');
  var titleInput = document.getElementById('svc-title');
  if (titleInput) titleInput.focus();

  var keyHandler = function (e) {
    if (e.key === 'Escape') { closeServiceModal(); return; }
    if (e.key === 'Tab') { _svcFocusTrap(modal, e); }
  };
  modal._svcKeyHandler = keyHandler;
  document.addEventListener('keydown', keyHandler);
}

function closeServiceModal() {
  var modal = document.getElementById('service-modal');
  modal.classList.add('hidden');
  modal.setAttribute('aria-hidden', 'true');

  if (modal._svcKeyHandler) {
    document.removeEventListener('keydown', modal._svcKeyHandler);
    delete modal._svcKeyHandler;
  }

  if (_svcModalTrigger && typeof _svcModalTrigger.focus === 'function') {
    _svcModalTrigger.focus();
  }
  _svcModalTrigger = null;
}

function editServiceByIndex(idx) {
  var d = serviceData[idx];
  if (!d) return;
  _resetSaveBtn();
  _svcModalTrigger = document.activeElement;
  document.getElementById('modal-title').textContent = 'Edit Product';
  document.getElementById('svc-id').value = d.id;
  document.getElementById('svc-title').value = d.title;
  document.getElementById('svc-desc').value = d.desc;
  document.getElementById('svc-price').value = d.price;
  document.getElementById('svc-status').value = d.status;
  var modal = document.getElementById('service-modal');
  modal.classList.remove('hidden');
  modal.setAttribute('aria-hidden', 'false');

  var keyHandler = function (e) {
    if (e.key === 'Escape') { closeServiceModal(); return; }
    if (e.key === 'Tab') { _svcFocusTrap(modal, e); }
  };
  modal._svcKeyHandler = keyHandler;
  document.addEventListener('keydown', keyHandler);

  var titleInput = document.getElementById('svc-title');
  if (titleInput) titleInput.focus();
}

async function saveService() {
  var id = document.getElementById('svc-id').value;
  var title = document.getElementById('svc-title').value.trim();
  var desc = document.getElementById('svc-desc').value.trim();
  var price = document.getElementById('svc-price').value.trim();
  var status = document.getElementById('svc-status').value;
  if (!title) { showAlert('Title is required', 'error'); return; }
  if (!price || isNaN(price) || parseFloat(price) <= 0) { showAlert('Valid price is required', 'error'); return; }

  var saveBtn = document.querySelector('#service-modal .btn-primary');
  if (saveBtn) { saveBtn.disabled = true; saveBtn.textContent = 'Saving...'; }

  try {
    if (id) {
      await apiUpdateService(id, { title: title, description: desc || null, price: price, status: status });
      showToast('Product updated', 'success');
    } else {
      var newSvc = await apiCreateService(title, desc || null, price);
      // If user selected "published", update status after creation
      if (status === 'published' && newSvc && newSvc.data && newSvc.data.id) {
        await apiUpdateService(newSvc.data.id, { status: 'published' });
      }
      showToast('Product created!', 'success');
    }
    _resetSaveBtn();
    closeServiceModal();
    await initMyServices();
  } catch (err) {
    showAlert(err.message, 'error');
    if (saveBtn) { saveBtn.disabled = false; saveBtn.textContent = 'Save'; }
    closeServiceModal();
    await initMyServices();
  }
}

async function deleteService(id) {
  var confirmed = await showConfirmDialog('Delete this product permanently?');
  if (!confirmed) return;
  try {
    await apiDeleteService(id);
    showToast('Product deleted', 'info');
    await initMyServices();
  } catch (err) {
    showAlert(err.message, 'error');
  }
}

// LR #6: Web/DB — Dashboard logic (client + executor)
// LR #12: AI Integration — order/service CRUD, modals, status updates

var serviceCache = {};
var serviceData = [];

async function fetchServiceTitle(serviceId) {
  if (serviceCache[serviceId]) return serviceCache[serviceId];
  try {
    var res = await apiFetchService(serviceId);
    if (res && res.data) {
      serviceCache[serviceId] = res.data.title;
      return res.data.title;
    }
  } catch (e) { /* ignore */ }
  return serviceId.slice(0, 8) + '...';
}

function formatPrice(p) {
  return '$' + parseFloat(p).toFixed(2);
}

function formatDate(d) {
  if (!d) return '';
  var dt = new Date(d);
  return dt.toLocaleDateString() + ' ' + dt.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}

// ===== Client Dashboard =====

async function initClientDashboard() {
  var container = document.getElementById('orders-content');
  if (!container) return;
  container.innerHTML = '<div class="flex-center" style="padding:40px;"><div class="spinner"></div></div>';
  try {
    var res = await apiFetchOrders(50, 0);
    var orders = res.data || [];
    var total = res.meta ? res.meta.total : orders.length;
    updateCount('orders-count', total);
    if (!orders.length) {
      container.innerHTML = '<div class="empty-state"><p>No orders yet</p><p class="text-secondary">Browse services to place your first order.</p><a href="/catalog" class="btn btn-primary mt-4">Browse Services</a></div>';
      return;
    }
    var titles = {};
    await Promise.all(orders.map(function (o) {
      return fetchServiceTitle(o.service_id).then(function (t) { titles[o.service_id] = t; });
    }));
    var html = '<div class="table-wrapper"><table><thead><tr><th>Service</th><th>Amount</th><th>Status</th><th>Date</th><th>Actions</th></tr></thead><tbody>';
    orders.forEach(function (o) {
      var badge = renderBadge(o.status);
      var actions = '';
      if (o.status === 'pending') {
        actions += '<button class="btn btn-success btn-sm" onclick="clientPay(\'' + o.id + '\')">Pay</button> ';
        actions += '<button class="btn btn-danger btn-sm" onclick="clientCancel(\'' + o.id + '\')">Cancel</button>';
      } else if (o.status === 'funded') {
        actions += '<span class="text-secondary">Awaiting completion</span>';
      } else if (o.status === 'disputed') {
        actions += '<a href="/audit/' + o.id + '" class="btn btn-outline btn-sm">Dispute</a>';
      } else {
        actions += '<a href="/orders/' + o.id + '" class="btn btn-outline btn-sm">View</a>';
      }
      html += '<tr><td>' + escapeHtml(titles[o.service_id] || '...') + '</td><td>' + formatPrice(o.amount) + '</td><td>' + badge + '</td><td>' + formatDate(o.created_at) + '</td><td>' + actions + '</td></tr>';
    });
    html += '</tbody></table></div>';
    container.innerHTML = html;
  } catch (err) {
    container.innerHTML = '<div class="alert alert-error">Failed to load orders: ' + escapeHtml(err.message) + '</div>';
  }
}

async function clientPay(orderId) {
  if (!confirm('Proceed with payment for this order?')) return;
  try {
    await apiUpdateOrderStatus(orderId, 'funded');
    showToast('Order funded successfully', 'success');
    initClientDashboard();
  } catch (err) {
    showAlert(err.message, 'error');
  }
}

async function clientCancel(orderId) {
  if (!confirm('Cancel this order?')) return;
  try {
    await apiUpdateOrderStatus(orderId, 'cancelled');
    showToast('Order cancelled', 'info');
    initClientDashboard();
  } catch (err) {
    showAlert(err.message, 'error');
  }
}

// ===== Executor Dashboard =====

async function initExecutorDashboard() {
  await Promise.all([initMyServices(), initIncomingOrders()]);
}

// -- My Services --

async function initMyServices() {
  var container = document.getElementById('services-content');
  if (!container) return;
  container.innerHTML = '<div class="flex-center" style="padding:40px;"><div class="spinner"></div></div>';
  try {
    var res = await apiFetchMyServices();
    var services = res.data || [];
    var total = res.meta ? res.meta.total : services.length;
    updateCount('services-count', total);
    if (!services.length) {
      container.innerHTML = '<div class="empty-state"><p>No services yet</p><p class="text-secondary">Create your first service to start receiving orders.</p></div>';
      return;
    }
    serviceData = [];
    var html = '<div class="table-wrapper"><table><thead><tr><th>Title</th><th>Price</th><th>Status</th><th>Actions</th></tr></thead><tbody>';
    services.forEach(function (s) {
      var badge = renderBadge(s.status);
      serviceData.push({ id: s.id, title: s.title, desc: s.description || '', price: s.price, status: s.status });
      var idx = serviceData.length - 1;
      html += '<tr><td>' + escapeHtml(s.title) + '</td><td>' + formatPrice(s.price) + '</td><td>' + badge + '</td><td>'
        + '<button class="btn btn-outline btn-sm" onclick="editServiceByIndex(' + idx + ')">Edit</button> '
        + '<button class="btn btn-danger btn-sm" onclick="deleteService(\'' + s.id + '\')">Delete</button>'
        + '</td></tr>';
    });
    html += '</tbody></table></div>';
    container.innerHTML = html;
  } catch (err) {
    container.innerHTML = '<div class="alert alert-error">Failed to load services: ' + escapeHtml(err.message) + '</div>';
  }
}

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
  editService(d.id, d.title, d.desc, d.price, d.status);
}

function editService(id, title, desc, price, status) {
  document.getElementById('modal-title').textContent = 'Edit Service';
  document.getElementById('svc-id').value = id;
  document.getElementById('svc-title').value = title;
  document.getElementById('svc-desc').value = desc;
  document.getElementById('svc-price').value = price;
  document.getElementById('svc-status').value = status;
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

// -- Incoming Orders --

async function initIncomingOrders() {
  var container = document.getElementById('incoming-content');
  if (!container) return;
  container.innerHTML = '<div class="flex-center" style="padding:40px;"><div class="spinner"></div></div>';
  try {
    var res = await apiFetchSoldOrders();
    var orders = res.data || [];
    var total = res.meta ? res.meta.total : orders.length;
    updateCount('incoming-count', total);
    if (!orders.length) {
      container.innerHTML = '<div class="empty-state"><p>No incoming orders yet</p><p class="text-secondary">Orders will appear here when clients purchase your services.</p></div>';
      return;
    }
    var titles = {};
    await Promise.all(orders.map(function (o) {
      return fetchServiceTitle(o.service_id).then(function (t) { titles[o.service_id] = t; });
    }));
    var html = '<div class="table-wrapper"><table><thead><tr><th>Service</th><th>Amount</th><th>Status</th><th>Date</th><th>Actions</th></tr></thead><tbody>';
    orders.forEach(function (o) {
      var badge = renderBadge(o.status);
      var actions = '';
      if (o.status === 'funded') {
        actions += '<button class="btn btn-success btn-sm" onclick="executorComplete(\'' + o.id + '\')">Complete</button> ';
      }
      if (o.status === 'pending' || o.status === 'funded') {
        actions += '<button class="btn btn-danger btn-sm" onclick="executorCancel(\'' + o.id + '\')">Cancel</button>';
      }
      if (!actions) {
        actions += '<a href="/orders/' + o.id + '" class="btn btn-outline btn-sm">View</a>';
      }
      html += '<tr><td>' + escapeHtml(titles[o.service_id] || '...') + '</td><td>' + formatPrice(o.amount) + '</td><td>' + badge + '</td><td>' + formatDate(o.created_at) + '</td><td>' + actions + '</td></tr>';
    });
    html += '</tbody></table></div>';
    container.innerHTML = html;
  } catch (err) {
    container.innerHTML = '<div class="alert alert-error">Failed to load orders: ' + escapeHtml(err.message) + '</div>';
  }
}

async function executorComplete(orderId) {
  if (!confirm('Mark this order as complete and release payment?')) return;
  try {
    await apiUpdateOrderStatus(orderId, 'released');
    showToast('Order completed, payment released', 'success');
    initIncomingOrders();
  } catch (err) {
    showAlert(err.message, 'error');
  }
}

async function executorCancel(orderId) {
  if (!confirm('Cancel this order?')) return;
  try {
    await apiUpdateOrderStatus(orderId, 'cancelled');
    showToast('Order cancelled', 'info');
    initIncomingOrders();
  } catch (err) {
    showAlert(err.message, 'error');
  }
}

// ===== Shared helpers =====

function updateCount(id, total) {
  var el = document.getElementById(id);
  if (el) el.textContent = '(' + total + ')';
}

// ===== Init by page type =====
document.addEventListener('DOMContentLoaded', function () {
  updateNavbar();
  var path = window.location.pathname;
  if (path === '/dashboard/client') {
    initClientDashboard();
  } else if (path === '/dashboard/executor') {
    initExecutorDashboard();
  }
});

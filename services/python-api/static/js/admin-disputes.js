// Admin dispute management — list + resolve

var _disputeActionInProgress = {};
var _selectedOrder = null;

function initAdminDisputes() {
  var loadingEl = document.getElementById('disputes-loading');
  var emptyEl = document.getElementById('disputes-empty');
  var errEl = document.getElementById('disputes-error');
  var tableEl = document.getElementById('disputes-table-wrapper');
  var tbody = document.getElementById('disputes-tbody');

  if (!tbody) return;

  loadingEl.style.display = 'block';
  emptyEl.style.display = 'none';
  errEl.style.display = 'none';
  tableEl.style.display = 'none';

  var auth = checkAuth();
  if (!auth.isAuthenticated) {
    window.location.href = '/login';
    return;
  }
  if (auth.role !== 'admin') {
    window.location.href = '/dashboard';
    return;
  }

  apiFetch('/admin/disputes?limit=200&offset=0', {}, 1)
    .then(function (res) {
      loadingEl.style.display = 'none';
      var orders = res.data || [];
      if (!orders.length) {
        emptyEl.style.display = 'block';
        return;
      }
      renderDisputes(tbody, orders);
      tableEl.style.display = 'block';
    })
    .catch(function (err) {
      loadingEl.style.display = 'none';
      errEl.style.display = 'block';
      errEl.innerHTML = '<div class="alert alert-error">Failed to load disputes: ' + escapeHtml(err.message || 'Unknown error') + '</div>';
    });
}

function renderDisputes(tbody, orders) {
  var html = '';
  orders.forEach(function (o) {
    var reason = o.notes || '—';
    html += '<tr>'
      + '<td data-label="Order ID" style="font-family:monospace;font-size:0.85rem;">' + escapeHtml(o.id.slice(0, 8)) + '...</td>'
      + '<td data-label="Buyer">' + escapeHtml(o.buyer_email || o.buyer_id.slice(0, 8) + '...') + '</td>'
      + '<td data-label="Seller">' + escapeHtml(o.seller_email || o.seller_id.slice(0, 8) + '...') + '</td>'
      + '<td data-label="Amount">$' + parseFloat(o.amount || 0).toFixed(2) + '</td>'
      + '<td data-label="Status">' + statusBadge(o.status) + '</td>'
      + '<td data-label="Actions" id="dactions-' + o.id + '">'
      + '<button type="button" class="btn-action btn-escrow-release btn-sm" onclick="handleResolve(\'' + o.id + '\',\'release\')">&#128184; Release</button> '
      + '<button type="button" class="btn-action btn-action-cancel btn-sm" onclick="handleResolve(\'' + o.id + '\',\'refund\')">&#128184; Refund</button> '
      + '<button type="button" class="btn-action btn-action-secondary btn-sm" onclick="showDetails(\'' + o.id + '\')">&#128269; Details</button>'
      + '</td>'
      + '</tr>';
  });
  tbody.innerHTML = html;
}

function showDetails(orderId) {
  apiFetch('/admin/disputes?limit=200&offset=0', {}, 1)
    .then(function (res) {
      var orders = res.data || [];
      var o = orders.find(function (item) { return item.id === orderId; });
      if (!o) {
        showToast('Order not found', 'error');
        return;
      }
      renderDetailsModal(o);
    })
    .catch(function () {
      showToast('Failed to load order details', 'error');
    });
}

function renderDetailsModal(o) {
  var overlay = document.getElementById('modal-overlay');
  if (!overlay) {
    overlay = document.createElement('div');
    overlay.id = 'modal-overlay';
    overlay.className = 'modal-overlay';
    document.body.appendChild(overlay);
  }

  var reason = o.notes || '—';

  overlay.innerHTML = ''
    + '<div class="modal-container">'
    + '<div class="modal-header"><h3>&#128269; Order Details</h3></div>'
    + '<div class="modal-body">'
    + '<div class="detail-row"><span class="detail-label">Order ID</span><span class="detail-value" style="font-family:monospace;">' + escapeHtml(o.id) + '</span></div>'
    + '<div class="detail-row"><span class="detail-label">Buyer</span><span class="detail-value">' + escapeHtml(o.buyer_email || o.buyer_id) + '</span></div>'
    + '<div class="detail-row"><span class="detail-label">Seller</span><span class="detail-value">' + escapeHtml(o.seller_email || o.seller_id) + '</span></div>'
    + '<div class="detail-row"><span class="detail-label">Amount</span><span class="detail-value">$' + parseFloat(o.amount || 0).toFixed(2) + '</span></div>'
    + '<div class="detail-row"><span class="detail-label">Status</span><span class="detail-value">' + statusBadge(o.status) + '</span></div>'
    + '<div class="detail-row"><span class="detail-label">Dispute Reason</span><span class="detail-value">' + escapeHtml(reason) + '</span></div>'
    + '<div class="detail-row"><span class="detail-label">Created</span><span class="detail-value">' + escapeHtml(o.created_at || '—') + '</span></div>'
    + '<div class="detail-row"><span class="detail-label">Updated</span><span class="detail-value">' + escapeHtml(o.updated_at || '—') + '</span></div>'
    + '</div>'
    + '<div class="modal-footer"><button type="button" class="btn btn-primary" onclick="closeDetailsModal()">Close</button></div>'
    + '</div>';

  overlay.style.display = 'flex';
}

function closeDetailsModal() {
  var overlay = document.getElementById('modal-overlay');
  if (overlay) overlay.style.display = 'none';
}

async function handleResolve(orderId, action) {
  if (_disputeActionInProgress[orderId]) return;
  _disputeActionInProgress[orderId] = true;

  var label = action === 'refund' ? 'refund buyer' : 'release to provider';
  var confirmed = await showConfirmDialog('Resolve dispute — ' + label + ' for order ' + orderId.slice(0, 8) + '...?');
  if (!confirmed) {
    delete _disputeActionInProgress[orderId];
    return;
  }

  var actionsEl = document.getElementById('dactions-' + orderId);
  if (actionsEl) {
    actionsEl.querySelectorAll('button').forEach(function (b) { b.disabled = true; });
  }

  try {
    await apiEscrowResolve(orderId, action);
    showToast('Dispute resolved — ' + label, 'success');

    var row = actionsEl ? actionsEl.closest('tr') : null;
    if (row) {
      var statusCell = row.querySelector('[data-label="Status"]');
      if (statusCell) {
        var newStatus = action === 'refund' ? 'resolved_refund' : 'resolved_release';
        statusCell.innerHTML = statusBadge(newStatus);
      }
      if (actionsEl) {
        actionsEl.innerHTML = '<span class="text-secondary">Resolved</span>';
      }
    }

    closeDetailsModal();
  } catch (err) {
    showToast(err.message || 'Failed to resolve dispute', 'error');
    if (actionsEl) {
      actionsEl.querySelectorAll('button').forEach(function (b) { b.disabled = false; });
    }
  }

  delete _disputeActionInProgress[orderId];
}

document.addEventListener('click', function (e) {
  var overlay = document.getElementById('modal-overlay');
  if (overlay && overlay.style.display === 'flex' && e.target === overlay) {
    closeDetailsModal();
  }
});

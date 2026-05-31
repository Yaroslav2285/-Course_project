// LR #6: Web/DB — Escrow panel with state-machine visualization (Phase 7.4)
// LR #10: Multi-lang/REST — escrow actions via proxy with idempotency
// LR #12: AI Integration — skeleton, error, optimistic UI, debounce
// LR #14: Data Engineering — escrow state mapping

var ESCROW_STEPS = [
  { status: 'pending',      label: 'Created',        icon: '&#128196;' },
  { status: 'funded',       label: 'Funded',         icon: '&#128176;' },
  { status: 'in_progress',  label: 'In Progress',    icon: '&#9881;&#65039;' },
  { status: 'completed',    label: 'Completed',      icon: '&#9989;' },
  { status: 'released',     label: 'Released',       icon: '&#128184;' },
];

var _escrowActionInProgress = {};

function getEscrowStepIndex(status) {
  for (var i = 0; i < ESCROW_STEPS.length; i++) {
    if (ESCROW_STEPS[i].status === status) return i;
  }
  return -1;
}

function renderEscrowTimeline(currentStatus, disputed) {
  var currentIdx = getEscrowStepIndex(currentStatus);
  if (currentIdx === -1) currentIdx = 0;

  var html = '<div class="escrow-timeline">';
  ESCROW_STEPS.forEach(function (step, i) {
    var cls = 'escrow-step';
    if (i < currentIdx) cls += ' completed';
    else if (i === currentIdx) cls += ' active';
    if (disputed && i === currentIdx) cls += ' disputed';
    html += '<div class="' + cls + '">'
      + '<div class="step-label">' + step.icon + ' ' + step.label + '</div>'
      + '</div>';
  });
  if (disputed && currentIdx < ESCROW_STEPS.length - 1) {
    html += '<div class="escrow-step disputed">'
      + '<div class="step-label">&#9888;&#65039; Disputed</div>'
      + '</div>';
  }
  html += '</div>';
  return html;
}

function renderEscrowActions(status, orderId, role) {
  var html = '';
  switch (status) {
    case 'pending':
      if (role === 'client') {
        html += '<button class="btn-action btn-escrow-fund" onclick="handleEscrowPanelAction(\'' + orderId + '\',\'fund\')">&#128176; Pay (Fund Escrow)</button>';
        html += '<button class="btn-action btn-action-cancel" onclick="handleEscrowPanelAction(\'' + orderId + '\',\'cancel\')">Cancel Order</button>';
      }
      break;
    case 'funded':
      if (role === 'provider') {
        html += '<button class="btn-action btn-escrow-advance" onclick="handleEscrowPanelAction(\'' + orderId + '\',\'advance\')">&#128640; Accept & Start</button>';
        html += '<button class="btn-action btn-action-cancel" onclick="handleEscrowPanelAction(\'' + orderId + '\',\'cancel\')">Cancel</button>';
      }
      break;
    case 'in_progress':
      if (role === 'provider') {
        html += '<button class="btn-action btn-escrow-complete" onclick="handleEscrowPanelAction(\'' + orderId + '\',\'complete\')">&#9989; Complete Work</button>';
      }
      if (role === 'client') {
        html += '<button class="btn-action btn-action-dispute" onclick="handleEscrowPanelAction(\'' + orderId + '\',\'dispute\')">&#9888;&#65039; Open Dispute</button>';
      }
      break;
    case 'completed':
      if (role === 'client') {
        html += '<button class="btn-action btn-escrow-release" onclick="handleEscrowPanelAction(\'' + orderId + '\',\'release\')">&#128184; Confirm & Release Payment</button>';
        html += '<button class="btn-action btn-action-dispute" onclick="handleEscrowPanelAction(\'' + orderId + '\',\'dispute\')">&#9888;&#65039; Open Dispute</button>';
      }
      break;
    case 'released':
      html += '<div class="alert alert-success" style="margin:0;">&#9989; Order completed. Payment released to executor.</div>';
      break;
    case 'disputed':
      html += '<div class="alert alert-warning" style="margin:0;">&#9888;&#65039; Dispute opened. Admin will review.</div>';
      break;
    case 'cancelled':
      html += '<div class="alert alert-info" style="margin:0;">Order cancelled.</div>';
      break;
    default:
      html += '<span class="text-secondary">No actions available</span>';
  }
  return html;
}

async function loadEscrowPanel(orderId) {
  var container = document.getElementById('escrow-panel-container');
  if (!container) return;

  container.innerHTML = '<div class="escrow-panel-skeleton"><div class="flex-center"><div class="spinner"></div></div></div>';

  try {
    var order = await apiClient('/orders/' + orderId, {}, 1);
    var orderData = order.data || {};

    var escrow = null;
    try {
      escrow = await apiEscrowByOrder(orderId);
    } catch (e) {
      // escrow unavailable — continue with order data
    }

    var status = escrow && escrow.status ? escrow.status : (orderData.status || 'pending');
    var disputed = status === 'disputed';
    var role = checkAuth().role || 'client';

    var infoGrid = document.getElementById('order-info-grid');
    var infoHtml = ''
      + '<div class="escrow-info-item"><div class="label">Order ID</div><div class="value" style="font-size:0.85rem;font-family:monospace;">' + escapeHtml(orderId) + '</div></div>'
      + '<div class="escrow-info-item"><div class="label">Amount</div><div class="value">$' + parseFloat(orderData.amount || 0).toFixed(2) + '</div></div>'
      + '<div class="escrow-info-item"><div class="label">Status</div><div class="value">' + statusBadge(status) + '</div></div>'
      + '<div class="escrow-info-item"><div class="label">Created</div><div class="value">' + formatDate(orderData.created_at) + '</div></div>';
    infoGrid.innerHTML = infoHtml;

    var timelineHtml = '<div class="escrow-panel">'
      + '<h3>&#128220; Escrow Status</h3>'
      + renderEscrowTimeline(status, disputed)
      + '<div class="escrow-actions" id="escrow-actions">'
      + renderEscrowActions(status, orderId, role)
      + '</div>'
      + '</div>';
    container.innerHTML = timelineHtml;

    var auditLink = document.getElementById('audit-link');
    if (auditLink) {
      auditLink.href = '/audit/' + orderId;
    }
  } catch (err) {
    container.innerHTML = '<div class="card" style="padding:16px;margin-top:16px;"><div class="alert alert-error">Failed to load escrow data: ' + escapeHtml(err.message || 'Unknown error') + '</div></div>';
  }
}

async function handleEscrowPanelAction(orderId, action) {
  if (_escrowActionInProgress[orderId]) return;
  _escrowActionInProgress[orderId] = true;

  var confirmMsg = '';
  switch (action) {
    case 'fund': confirmMsg = 'Pay $' + ' and fund escrow?'; break;
    case 'advance': confirmMsg = 'Accept this order and start working?'; break;
    case 'complete': confirmMsg = 'Mark work as complete?'; break;
    case 'release': confirmMsg = 'Confirm release of funds?'; break;
    case 'dispute': confirmMsg = 'Open a dispute?'; break;
    case 'cancel': confirmMsg = 'Cancel this order?'; break;
  }
  if (confirmMsg) {
    var confirmed = await showConfirmDialog(confirmMsg);
    if (!confirmed) {
      delete _escrowActionInProgress[orderId];
      return;
    }
  }

  var actionsEl = document.getElementById('escrow-actions');
  var buttons = actionsEl ? actionsEl.querySelectorAll('.btn-action') : [];
  buttons.forEach(function (b) { b.disabled = true; });

  try {
    if (action === 'cancel') {
      await apiUpdateOrderStatus(orderId, 'cancelled');
      showToast('Order cancelled', 'info');
    } else if (action === 'fund') {
      try {
        await apiEscrowAction(orderId, 'fund');
      } catch (e) {
        if (e.status === 404 || e.status === 0) {
          await apiUpdateOrderStatus(orderId, 'funded');
        } else { throw e; }
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
        } else { throw e; }
      }
      showToast('Work marked complete!', 'success');
    } else if (action === 'release') {
      try {
        await apiEscrowAction(orderId, 'release');
      } catch (e) {
        if (e.status === 404 || e.status === 0) {
          await apiUpdateOrderStatus(orderId, 'released');
        } else { throw e; }
      }
      showToast('Payment released!', 'success');
    } else if (action === 'dispute') {
      try {
        await apiEscrowAction(orderId, 'dispute');
      } catch (e) {
        if (e.status === 404 || e.status === 0) {
          await apiUpdateOrderStatus(orderId, 'disputed');
        } else { throw e; }
      }
      showToast('Dispute opened', 'info');
    }

    await loadEscrowPanel(orderId);
  } catch (err) {
    showToast(err.message || 'Action failed', 'error');
    buttons.forEach(function (b) { b.disabled = false; });
  }

  delete _escrowActionInProgress[orderId];
}

function initOrderDetailPage() {
  var pathParts = window.location.pathname.split('/');
  var orderId = pathParts[pathParts.length - 1];
  if (!orderId || orderId.length < 10) {
    var errorEl = document.getElementById('order-detail-error');
    var errorMsg = document.getElementById('order-detail-error-message');
    document.getElementById('order-detail-loading').classList.add('hidden');
    if (errorEl) {
      errorEl.classList.remove('hidden');
      if (errorMsg) errorMsg.textContent = 'Invalid order ID';
    }
    return;
  }

  loadOrderDetail();
}

async function loadOrderDetail() {
  var loadingEl = document.getElementById('order-detail-loading');
  var errorEl = document.getElementById('order-detail-error');
  var contentEl = document.getElementById('order-detail-content');
  var pathParts = window.location.pathname.split('/');
  var orderId = pathParts[pathParts.length - 1];

  if (loadingEl) loadingEl.classList.add('hidden');
  if (errorEl) errorEl.classList.add('hidden');
  if (contentEl) contentEl.classList.add('hidden');

  try {
    await loadEscrowPanel(orderId);
    if (contentEl) contentEl.classList.remove('hidden');
  } catch (err) {
    if (loadingEl) loadingEl.classList.add('hidden');
    if (errorEl) {
      errorEl.classList.remove('hidden');
      var errorMsg = document.getElementById('order-detail-error-message');
      if (errorMsg) errorMsg.textContent = err.message || 'Failed to load order';
    }
  }
}

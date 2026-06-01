// LR #6: Web/DB — Order creation modal (Phase 7.6)
// LR #10: Multi-lang/REST — creates order via API, 422 handling
// LR #12: AI Integration — validation, spinner, toast, redirect
// LR #15: Security/UX — focus trap, Escape key, return focus

var _orderModalState = null;
var _orderModalTrigger = null;

var ORDER_FOCUSABLE_SELECTOR = 'a[href], button:not([disabled]), textarea:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])';

function focusTrap(modalEl, event) {
  var focusable = modalEl.querySelectorAll(ORDER_FOCUSABLE_SELECTOR);
  if (focusable.length === 0) return;

  var first = focusable[0];
  var last = focusable[focusable.length - 1];

  if (event.key === 'Tab') {
    if (event.shiftKey) {
      if (document.activeElement === first) {
        event.preventDefault();
        last.focus();
      }
    } else {
      if (document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    }
  }
}

function updateOrderTotal() {
  var state = _orderModalState;
  if (!state) return;
  var qtyInput = document.getElementById('order-qty');
  var totalEl = document.getElementById('order-total-price');
  var unitPrice = parseFloat(state.price) || 0;
  var qty = parseInt(qtyInput.value, 10) || 1;
  if (qty < 1) qty = 1;
  if (qty > 99) qty = 99;
  qtyInput.value = qty;
  totalEl.textContent = '$' + (unitPrice * qty).toFixed(2);
}

function initQuantityControls() {
  var decBtn = document.getElementById('order-qty-dec');
  var incBtn = document.getElementById('order-qty-inc');
  var qtyInput = document.getElementById('order-qty');

  function onDec() {
    var val = parseInt(qtyInput.value, 10) || 1;
    if (val > 1) qtyInput.value = val - 1;
    updateOrderTotal();
  }

  function onInc() {
    var val = parseInt(qtyInput.value, 10) || 1;
    if (val < 99) qtyInput.value = val + 1;
    updateOrderTotal();
  }

  decBtn.addEventListener('click', onDec);
  incBtn.addEventListener('click', onInc);
  qtyInput.addEventListener('input', updateOrderTotal);
}

function openOrderModal(serviceId, providerId, price, serviceTitle) {
  _orderModalTrigger = document.activeElement;
  _orderModalState = { serviceId: serviceId, providerId: providerId, price: price };
  var modal = document.getElementById('order-modal');
  var error = document.getElementById('order-modal-error');
  error.classList.add('hidden');

  document.getElementById('order-product-title').textContent = escapeHtml(serviceTitle || 'Product');
  document.getElementById('order-unit-price').textContent = '$' + parseFloat(price).toFixed(2);

  var qtyInput = document.getElementById('order-qty');
  qtyInput.value = 1;
  updateOrderTotal();

  modal.classList.remove('hidden');
  modal.setAttribute('aria-hidden', 'false');

  var submitBtn = document.getElementById('order-modal-submit');
  if (submitBtn) submitBtn.focus();

  var keyHandler = function (e) {
    if (e.key === 'Escape') {
      closeOrderModal();
      return;
    }
    if (e.key === 'Tab') {
      focusTrap(modal, e);
    }
  };
  modal._keyHandler = keyHandler;
  document.addEventListener('keydown', keyHandler);
}

function closeOrderModal() {
  var modal = document.getElementById('order-modal');
  modal.classList.add('hidden');
  modal.setAttribute('aria-hidden', 'true');

  if (modal._keyHandler) {
    document.removeEventListener('keydown', modal._keyHandler);
    delete modal._keyHandler;
  }

  if (_orderModalTrigger && typeof _orderModalTrigger.focus === 'function') {
    _orderModalTrigger.focus();
  }
  _orderModalTrigger = null;
  _orderModalState = null;
}

async function submitOrderModal() {
  var state = _orderModalState;
  if (!state) return;

  var qtyInput = document.getElementById('order-qty');
  var qty = parseInt(qtyInput.value, 10) || 1;
  if (qty < 1) qty = 1;
  if (qty > 99) { qty = 99; qtyInput.value = 99; }

  var errorEl = document.getElementById('order-modal-error');
  errorEl.classList.add('hidden');

  var submitBtn = document.getElementById('order-modal-submit');
  var submitText = document.getElementById('order-modal-submit-text');
  var submitSpinner = document.getElementById('order-modal-submit-spinner');
  submitBtn.disabled = true;
  submitText.textContent = 'Placing Order...';
  submitSpinner.classList.remove('hidden');

  var user = getUser();
  if (!user || !user.id) {
    errorEl.textContent = 'User not authenticated. Please log in.';
    errorEl.classList.remove('hidden');
    submitBtn.disabled = false;
    submitText.textContent = 'Place Order';
    submitSpinner.classList.add('hidden');
    return;
  }

  try {
    await apiClient('/orders/', {
      method: 'POST',
      body: JSON.stringify({
        service_id: state.serviceId,
        buyer_id: user.id,
        seller_id: state.providerId,
        amount: (parseFloat(state.price) * qty).toFixed(4)
      })
    }, 1);

    showToast('Order placed successfully!', 'success');
    closeOrderModal();
    setTimeout(function () {
      window.location.href = '/dashboard/client?tab=orders';
    }, 1200);
  } catch (err) {
    var msg = err.message || 'Failed to place order';
    if (err.data && err.data.errors && err.data.errors[0]) {
      var e = err.data.errors[0];
      msg = e.detail || msg;
      if (e.fields && e.fields.length) {
        msg += ' (' + e.fields.map(function(f) { return f.field + ': ' + f.message; }).join('; ') + ')';
      }
    }
    errorEl.textContent = msg;
    errorEl.classList.remove('hidden');
    submitBtn.disabled = false;
    submitText.textContent = 'Place Order';
    submitSpinner.classList.add('hidden');
    errorEl.focus();
  }
}

// Init on DOM ready
if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', initQuantityControls);
} else {
  initQuantityControls();
}

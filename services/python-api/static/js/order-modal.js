// LR #6: Web/DB — Order creation modal (Phase 7.4)
// LR #10: Multi-lang/REST — creates order via API, 422 handling
// LR #12: AI Integration — validation, spinner, toast, redirect

var _orderModalState = null;

function openOrderModal(serviceId, providerId, price, serviceTitle) {
  _orderModalState = { serviceId: serviceId, providerId: providerId, price: price };
  var modal = document.getElementById('order-modal');
  var info = document.getElementById('order-modal-service');
  var error = document.getElementById('order-modal-error');
  error.classList.add('hidden');
  info.innerHTML = '<div class="order-modal-service-info-inner"><strong>' + escapeHtml(serviceTitle || 'Service') + '</strong> &mdash; <span class="text-price">$' + parseFloat(price).toFixed(2) + '</span></div>';

  var deadlineInput = document.getElementById('order-deadline');
  var tomorrow = new Date();
  tomorrow.setDate(tomorrow.getDate() + 1);
  deadlineInput.min = tomorrow.toISOString().split('T')[0];
  deadlineInput.value = '';

  document.getElementById('order-notes').value = '';
  document.getElementById('order-files').value = '';
  document.getElementById('order-offer-accept').checked = false;

  modal.classList.remove('hidden');
  document.getElementById('order-notes').focus();
}

function closeOrderModal() {
  document.getElementById('order-modal').classList.add('hidden');
  _orderModalState = null;
}

async function submitOrderModal() {
  var state = _orderModalState;
  if (!state) return;

  var errorEl = document.getElementById('order-modal-error');
  errorEl.classList.add('hidden');

  var notes = document.getElementById('order-notes').value.trim();
  var offerAccepted = document.getElementById('order-offer-accept').checked;

  if (!offerAccepted) {
    errorEl.textContent = 'Please accept the Offer Terms to proceed.';
    errorEl.classList.remove('hidden');
    return;
  }

  var submitBtn = document.getElementById('order-modal-submit');
  var submitText = document.getElementById('order-modal-submit-text');
  var submitSpinner = document.getElementById('order-modal-submit-spinner');
  submitBtn.disabled = true;
  submitText.textContent = 'Placing Order...';
  submitSpinner.classList.remove('hidden');

  try {
    await apiClient('/orders/', {
      method: 'POST',
      body: JSON.stringify({
        service_id: state.serviceId,
        seller_id: state.providerId,
        amount: parseFloat(state.price),
        notes: notes || ''
      })
    }, 1);

    showToast('Order created successfully!', 'success');
    closeOrderModal();
    setTimeout(function () {
      window.location.href = '/dashboard/client?tab=orders';
    }, 1200);
  } catch (err) {
    var msg = err.message || 'Failed to create order';
    if (err.data && err.data.errors && err.data.errors[0] && err.data.errors[0].detail) {
      msg = err.data.errors[0].detail;
    }
    errorEl.textContent = msg;
    errorEl.classList.remove('hidden');
    submitBtn.disabled = false;
    submitText.textContent = 'Place Order';
    submitSpinner.classList.add('hidden');
  }
}

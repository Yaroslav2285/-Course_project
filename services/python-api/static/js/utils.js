// LR #6: Web/DB — Utility functions for Service Marketplace UI
// LR #12: AI Integration — debounce, escapeHtml, renderBadge
// LR #15: Security/UX — showToast delegated to toast.js, backwards compatible

// === Toast notifications (provided by toast.js) ===
// showToast is defined in toast.js as window.showToast.
// Do NOT redeclare here — utils.js showToast was removed to prevent
// recursion (it was overwriting window.showToast and calling itself).

// === Custom confirm dialog (replaces browser confirm()) ===
function showConfirmDialog(message) {
  return new Promise(function (resolve) {
    var modal = document.getElementById('confirm-modal');
    var msgEl = document.getElementById('confirm-message');
    var cancelBtn = document.getElementById('confirm-cancel');
    var okBtn = document.getElementById('confirm-ok');

    if (!modal || !msgEl || !cancelBtn || !okBtn) {
      resolve(window.confirm(message));
      return;
    }

    msgEl.textContent = message;
    modal.classList.remove('hidden');
    modal._confirmResolve = resolve;

    function cleanup() {
      modal.classList.add('hidden');
      if (modal._keyHandler) {
        document.removeEventListener('keydown', modal._keyHandler);
        delete modal._keyHandler;
      }
      cancelBtn.removeEventListener('click', onCancel);
      okBtn.removeEventListener('click', onOk);
    }

    function onCancel() {
      cleanup();
      resolve(false);
    }

    function onOk() {
      cleanup();
      resolve(true);
    }

    cancelBtn.addEventListener('click', onCancel);
    okBtn.addEventListener('click', onOk);
    okBtn.focus();

    modal._keyHandler = function (e) {
      if (e.key === 'Escape') { onCancel(); return; }
      if (e.key === 'Tab') {
        var focusable = modal.querySelectorAll('a[href], button:not([disabled]), [tabindex]:not([tabindex="-1"])');
        if (focusable.length === 0) return;
        var first = focusable[0];
        var last = focusable[focusable.length - 1];
        if (e.shiftKey && document.activeElement === first) {
          e.preventDefault();
          last.focus();
        } else if (!e.shiftKey && document.activeElement === last) {
          e.preventDefault();
          first.focus();
        }
      }
    };
    document.addEventListener('keydown', modal._keyHandler);

    modal.addEventListener('click', function (e) {
      if (e.target === modal) onCancel();
    });
  });
}

// === Alert helper ===
function showAlert(message, type, containerId) {
  containerId = containerId || 'alert-container';
  type = type || 'error';
  var container = document.getElementById(containerId);
  if (!container) return;
  container.innerHTML =
    '<div class="alert alert-' +
    type +
    '">' +
    escapeHtml(message) +
    '</div>';
  setTimeout(function () {
    container.innerHTML = '';
  }, 5000);
}

function clearAlerts(containerId) {
  containerId = containerId || 'alert-container';
  var container = document.getElementById(containerId);
  if (container) container.innerHTML = '';
}

// === Debounce ===
function debounce(fn, delay) {
  delay = delay || 300;
  var timer;
  return function () {
    var context = this;
    var args = arguments;
    clearTimeout(timer);
    timer = setTimeout(function () {
      fn.apply(context, args);
    }, delay);
  };
}

// === HTML escape ===
function escapeHtml(text) {
  if (!text) return '';
  var div = document.createElement('div');
  div.textContent = text;
  return div.innerHTML;
}

// === Badge renderer ===
function renderBadge(status) {
  if (!status) return '';
  return '<span class="badge badge-' + status + '">' + escapeHtml(status) + '</span>';
}

// === Escrow badge (uppercase status) ===
function renderEscrowBadge(status) {
  if (!status) return '';
  var cls = status.toLowerCase().replace(/_/g, '-');
  return '<span class="badge badge-' + cls + '">' + escapeHtml(status) + '</span>';
}

// === Format price ===
function formatPrice(p) { return '$' + parseFloat(p).toFixed(2); }

// === Format date ===
function formatDate(d) {
  if (!d) return '';
  var dt = new Date(d);
  return dt.toLocaleDateString() + ' ' + dt.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}

// === Status badge ===
function statusBadge(status) {
  if (!status) return '';
  var cls = status === 'in_progress' ? 'in_progress' : status;
  return '<span class="order-badge status-' + cls + '">' + status.replace(/_/g, ' ') + '</span>';
}

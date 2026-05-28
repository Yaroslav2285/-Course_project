// LR #6: Web/DB — Utility functions for Service Marketplace UI
// LR #12: AI Integration — toast, debounce, escapeHtml, renderBadge

// === Toast notifications ===
function showToast(message, type) {
  type = type || 'success';
  let container = document.getElementById('toast-container');
  if (!container) {
    container = document.createElement('div');
    container.id = 'toast-container';
    container.className = 'toast-container';
    document.body.appendChild(container);
  }
  const toast = document.createElement('div');
  toast.className = 'toast toast-' + type;
  toast.textContent = message;
  container.appendChild(toast);
  setTimeout(function () {
    toast.style.opacity = '0';
    toast.style.transition = 'opacity 0.3s';
    setTimeout(function () {
      toast.remove();
    }, 300);
  }, 3500);
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

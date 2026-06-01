// LR #6: Web/DB — Client wallet module (Phase 7.6)
// LR #10: Multi-lang/REST — wallet API calls with auth & idempotency
// LR #12: AI Integration — debounce, polling
// LR #15: Security/UX — focus trap, double-click protect, error handling

var _walletState = {
  balance: 0,
  currency: 'RUB',
  transactions: [],
  loading: false
};

var _walletTopUpInProgress = false;
var _walletSyncInterval = null;
var _walletCurrentPage = 0;
var _WALLET_PAGE_SIZE = 5;

async function _walletFetchBalance() {
  var res = await apiFetch('/wallet/balance');
  return res.data || res;
}

async function _walletFetchTransactions(limit) {
  limit = limit || 20;
  var res = await apiFetch('/wallet/transactions?limit=' + limit);
  var data = res.data || res;
  if (Array.isArray(data)) {
    return { items: data, total: (res.meta && res.meta.total) || data.length };
  }
  return { items: data.items || [], total: data.total || 0 };
}

async function _walletTopUp(amount) {
  var res = await apiFetch('/wallet/topup', {
    method: 'POST',
    body: JSON.stringify({ amount: amount })
  });
  return res.data || res;
}

function _walletFormatAmount(amount) {
  return new Intl.NumberFormat('ru-RU', { style: 'currency', currency: _walletState.currency }).format(Math.abs(amount));
}

function _walletFormatDate(iso) {
  try {
    var d = new Date(iso);
    return d.toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit' });
  } catch (e) {
    return iso;
  }
}

function _walletTypeLabel(type) {
  switch (type) {
    case 'topup': return 'Пополнение';
    case 'escrow_fund': return 'Списание (эскроу)';
    case 'refund': return 'Возврат';
    default: return type;
  }
}

function _walletStatusClass(status) {
  switch (status) {
    case 'success': return 'status-success';
    case 'pending': return 'status-pending';
    case 'failed': return 'status-failed';
    default: return '';
  }
}

function _walletStatusLabel(status) {
  switch (status) {
    case 'success': return 'Успешно';
    case 'pending': return 'В обработке';
    case 'failed': return 'Ошибка';
    default: return status;
  }
}

function _walletDebounce(fn, ms) {
  var timer = null;
  return function () {
    var args = arguments;
    var self = this;
    if (timer) clearTimeout(timer);
    timer = setTimeout(function () { fn.apply(self, args); }, ms);
  };
}

// ===== Public API =====

async function loadWalletData() {
  if (_walletState.loading) return;
  _walletState.loading = true;
  var balanceEl = document.getElementById('wallet-balance-value');
  if (balanceEl) balanceEl.textContent = '...';

  try {
    var results = await Promise.all([
      _walletFetchBalance(),
      _walletFetchTransactions(20)
    ]);

    _walletState.balance = parseFloat(results[0].balance);
    _walletState.currency = results[0].currency || 'RUB';
    _walletState.transactions = results[1].items || [];

    renderWalletBalance();
    renderWalletHistory();
  } catch (err) {
    console.error('Wallet load error:', err);
    renderWalletBalanceFallback();
    showToast('Ошибка загрузки кошелька: ' + (err.message || 'Unknown'), 'error');
  } finally {
    _walletState.loading = false;
  }
}

function renderWalletBalance() {
  var valueEl = document.getElementById('wallet-balance-value');
  var skeletonEl = document.getElementById('wallet-balance-skeleton');
  var contentEl = document.getElementById('wallet-balance-content');

  if (skeletonEl) skeletonEl.style.display = 'none';
  if (contentEl) contentEl.style.display = 'block';

  if (valueEl) {
    valueEl.textContent = _walletFormatAmount(_walletState.balance);
    valueEl.style.color = _walletState.balance >= 0 ? 'var(--color-success)' : 'var(--color-error)';
  }
}

function renderWalletBalanceFallback() {
  var skeletonEl = document.getElementById('wallet-balance-skeleton');
  var contentEl = document.getElementById('wallet-balance-content');
  var valueEl = document.getElementById('wallet-balance-value');
  if (skeletonEl) skeletonEl.style.display = 'none';
  if (contentEl) contentEl.style.display = 'block';
  if (valueEl) {
    valueEl.textContent = '0 ₽';
    valueEl.style.color = '';
  }
}

function renderWalletHistory() {
  var container = document.getElementById('wallet-history-body');
  var emptyEl = document.getElementById('wallet-history-empty');
  var moreBtn = document.getElementById('wallet-history-more');
  if (!container) return;

  var txns = _walletState.transactions;
  if (!txns || !txns.length) {
    if (emptyEl) emptyEl.style.display = 'block';
    if (moreBtn) moreBtn.style.display = 'none';
    container.innerHTML = '';
    return;
  }

  if (emptyEl) emptyEl.style.display = 'none';

  var start = 0;
  var end = (_walletCurrentPage + 1) * _WALLET_PAGE_SIZE;
  var pageTxns = txns.slice(start, end);

  var html = '';
  pageTxns.forEach(function (txn) {
    var txnAmount = parseFloat(txn.amount);
    var isNegative = txnAmount < 0;
    var sign = txnAmount >= 0 ? '+' : '';
    var amountClass = isNegative ? 'wallet-amount-negative' : 'wallet-amount-positive';
    html += '<tr class="wallet-history-row">'
      + '<td data-label="Дата">' + _walletFormatDate(txn.created_at) + '</td>'
      + '<td data-label="Тип">' + _walletTypeLabel(txn.type) + '</td>'
      + '<td data-label="Сумма" class="' + amountClass + '">' + sign + _walletFormatAmount(txnAmount) + '</td>'
      + '<td data-label="Статус"><span class="wallet-status-badge ' + _walletStatusClass(txn.status) + '">' + _walletStatusLabel(txn.status) + '</span></td>'
      + '<td data-label="ID" class="wallet-txn-id">' + escapeHtml(txn.id) + '</td>'
      + '</tr>';
  });

  container.innerHTML = html;

  if (moreBtn) {
    if (end >= txns.length) {
      moreBtn.style.display = 'none';
    } else {
      moreBtn.style.display = 'inline-flex';
      moreBtn.onclick = function () {
        _walletCurrentPage++;
        renderWalletHistory();
      };
    }
  }
}

async function handleTopUp(event) {
  event.preventDefault();
  if (_walletTopUpInProgress) return;
  _walletTopUpInProgress = true;

  var errorEl = document.getElementById('topup-error');
  var amountInput = document.getElementById('topup-amount');
  var submitBtn = document.getElementById('topup-submit');
  var submitText = document.getElementById('topup-submit-text');
  var submitSpinner = document.getElementById('topup-submit-spinner');

  if (errorEl) errorEl.classList.add('hidden');
  if (submitBtn) submitBtn.disabled = true;
  if (submitText) submitText.textContent = 'Пополнение...';
  if (submitSpinner) submitSpinner.classList.remove('hidden');

  var amountStr = amountInput ? amountInput.value.trim() : '';
  var amount = parseFloat(amountStr);

  if (!amountStr || isNaN(amount) || amount <= 0) {
    if (errorEl) {
      errorEl.textContent = 'Введите сумму больше 0';
      errorEl.classList.remove('hidden');
    }
    _walletTopUpInProgress = false;
    if (submitBtn) submitBtn.disabled = false;
    if (submitText) submitText.textContent = 'Пополнить';
    if (submitSpinner) submitSpinner.classList.add('hidden');
    return;
  }

  amount = Math.round(amount * 100) / 100;

  try {
    await _walletTopUp(amount);
    showToast('Кошелёк пополнен на ' + _walletFormatAmount(amount), 'success');
    closeTopUpModal();
    await loadWalletData();
  } catch (err) {
    if (errorEl) {
      errorEl.textContent = err.message || 'Ошибка пополнения';
      errorEl.classList.remove('hidden');
    }
  } finally {
    _walletTopUpInProgress = false;
    if (submitBtn) submitBtn.disabled = false;
    if (submitText) submitText.textContent = 'Пополнить';
    if (submitSpinner) submitSpinner.classList.add('hidden');
  }
}

function openTopUpModal() {
  var modal = document.getElementById('topup-modal');
  var amountInput = document.getElementById('topup-amount');
  var errorEl = document.getElementById('topup-error');

  if (!modal) return;

  if (errorEl) errorEl.classList.add('hidden');
  if (amountInput) amountInput.value = '';
  modal.classList.remove('hidden');
  modal.setAttribute('aria-hidden', 'false');
  if (amountInput) amountInput.focus();

  var keyHandler = function (e) {
    if (e.key === 'Escape') { closeTopUpModal(); return; }
    if (e.key === 'Tab') {
      var focusable = modal.querySelectorAll('a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])');
      if (!focusable.length) return;
      var first = focusable[0];
      var last = focusable[focusable.length - 1];
      if (e.shiftKey && document.activeElement === first) {
        e.preventDefault(); last.focus();
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault(); first.focus();
      }
    }
  };
  modal._walletKeyHandler = keyHandler;
  document.addEventListener('keydown', keyHandler);
}

function closeTopUpModal() {
  var modal = document.getElementById('topup-modal');
  if (!modal) return;
  modal.classList.add('hidden');
  modal.setAttribute('aria-hidden', 'true');

  if (modal._walletKeyHandler) {
    document.removeEventListener('keydown', modal._walletKeyHandler);
    delete modal._walletKeyHandler;
  }

  var trigger = document.getElementById('wallet-topup-btn');
  if (trigger && typeof trigger.focus === 'function') trigger.focus();
}

function startWalletSync() {
  if (_walletSyncInterval) return;
  _walletSyncInterval = setInterval(function () {
    if (document.hidden) return;
    loadWalletData();
  }, 5000);
}

function stopWalletSync() {
  if (_walletSyncInterval) {
    clearInterval(_walletSyncInterval);
    _walletSyncInterval = null;
  }
}

function initWallet() {
  var auth = checkAuth();
  if (!auth.isAuthenticated) return;

  loadWalletData();
  startWalletSync();
}

// Debounced amount validation
var _walletValidateAmount = _walletDebounce(function () {
  var input = document.getElementById('topup-amount');
  var errorEl = document.getElementById('topup-error');
  if (!input) return;
  var val = parseFloat(input.value);
  if (input.value && (isNaN(val) || val <= 0)) {
    if (errorEl) {
      errorEl.textContent = 'Введите корректную сумму';
      errorEl.classList.remove('hidden');
    }
  } else {
    if (errorEl) errorEl.classList.add('hidden');
  }
}, 300);

// Topup amount input listener — init on DOM ready
if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', function () {
    var input = document.getElementById('topup-amount');
    if (input) input.addEventListener('input', _walletValidateAmount);
  });
} else {
  var input = document.getElementById('topup-amount');
  if (input) input.addEventListener('input', _walletValidateAmount);
}

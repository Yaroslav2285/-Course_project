// LR #7: Frontend/UI
// LR #6: Web/DB — Blockchain audit page (Phase 7.4)
// LR #10: Multi-lang/REST — fetch audit via chain proxy
// LR #12: AI Integration — skeleton, empty, error, integrity check
// LR #14: Data Engineering/Hashing — block chain verification

function extractOrderIdFromPath() {
  var parts = window.location.pathname.split('/');
  return parts[parts.length - 1] || '';
}

function renderBlock(block, index, prevBlock) {
  var integrityValid = true;
  if (prevBlock && block.previous_hash !== prevBlock.hash) {
    integrityValid = false;
  }

  var blockCls = 'chain-block' + (integrityValid ? ' integrity-valid' : ' integrity-invalid');

  var dataHtml = '';
  var data = block.data || {};
  Object.keys(data).forEach(function (key) {
    dataHtml += '<dt>' + escapeHtml(key.replace(/_/g, ' ')) + '</dt>'
      + '<dd>' + escapeHtml(String(data[key])) + '</dd>';
  });

  return '<div class="' + blockCls + '">'
    + '<div class="block-header">'
    + '<span class="block-index">Block #' + block.index + '</span>'
    + '<span class="block-timestamp">' + formatDate(block.timestamp) + '</span>'
    + '</div>'
    + '<dl class="block-data">' + dataHtml + '</dl>'
    + '<div class="block-hash">'
    + '<span class="hash-label">' + (integrityValid ? '&#9989;' : '&#10060;') + ' Hash:</span>'
    + '<span class="hash-value">' + escapeHtml(block.hash || '') + '</span>'
    + '<button class="btn-copy-hash" onclick="copyHash(this, \'' + escapeHtml(String(block.hash)) + '\')">Copy</button>'
    + '</div>'
    + '</div>';
}

function renderConnector(valid) {
  var cls = valid ? 'connector-valid' : 'connector-invalid';
  return '<div class="block-connector"><span class="' + cls + '">&#11015;</span></div>';
}

function copyHash(btn, hash) {
  navigator.clipboard.writeText(hash).then(function () {
    btn.textContent = 'Copied!';
    setTimeout(function () { btn.textContent = 'Copy'; }, 2000);
  }).catch(function () {
    showToast('Failed to copy hash', 'error');
  });
}

async function loadAudit() {
  var orderId = extractOrderIdFromPath();
  if (!orderId) {
    showError('Invalid order ID');
    return;
  }

  var loadingEl = document.getElementById('audit-loading');
  var emptyEl = document.getElementById('audit-empty');
  var errorEl = document.getElementById('audit-error');
  var contentEl = document.getElementById('audit-content');
  var integrityEl = document.getElementById('audit-integrity');
  var chainBlocks = document.getElementById('chain-blocks');

  if (loadingEl) loadingEl.classList.remove('hidden');
  if (emptyEl) emptyEl.classList.add('hidden');
  if (errorEl) errorEl.classList.add('hidden');
  if (contentEl) contentEl.classList.add('hidden');

  try {
    var auditData = await apiClient('/chain/audit/' + orderId, {}, 1);
    var blocks = (auditData && auditData.data && auditData.data.blocks) || [];
    var orderIdStr = (auditData && auditData.data && auditData.data.order_id) || orderId;

    if (loadingEl) loadingEl.classList.add('hidden');

    if (!blocks.length) {
      if (emptyEl) emptyEl.classList.remove('hidden');
      return;
    }

    // Verify chain integrity
    var allValid = true;
    var blockHtml = '';
    for (var i = 0; i < blocks.length; i++) {
      var prevBlock = i > 0 ? blocks[i - 1] : null;
      blockHtml += renderBlock(blocks[i], i, prevBlock);
      if (i < blocks.length - 1) {
        var connectorValid = blocks[i + 1].previous_hash === blocks[i].hash;
        blockHtml += renderConnector(connectorValid);
        if (!connectorValid) allValid = false;
      }
    }

    chainBlocks.innerHTML = blockHtml;
    if (contentEl) contentEl.classList.remove('hidden');

    // Integrity badge
    if (integrityEl) {
      if (allValid) {
        integrityEl.className = 'audit-integrity-badge valid';
        integrityEl.innerHTML = '&#9989; Chain Integrity: Valid';
      } else {
        integrityEl.className = 'audit-integrity-badge invalid';
        integrityEl.innerHTML = '&#10060; Chain Integrity: Broken';
      }
    }
  } catch (err) {
    if (loadingEl) loadingEl.classList.add('hidden');
    if (errorEl) {
      errorEl.classList.remove('hidden');
      var errorMsg = document.getElementById('audit-error-message');
      if (errorMsg) errorMsg.textContent = err.message || 'Failed to load audit data';
    }
  }
}

function showError(msg) {
  var loadingEl = document.getElementById('audit-loading');
  var errorEl = document.getElementById('audit-error');
  if (loadingEl) loadingEl.classList.add('hidden');
  if (errorEl) {
    errorEl.classList.remove('hidden');
    var errorMsg = document.getElementById('audit-error-message');
    if (errorMsg) errorMsg.textContent = msg;
  }
}

function initAuditPage() {
  loadAudit();
}

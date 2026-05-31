// LR #6: Web/DB — Catalog page: filters, grid, pagination
// LR #10: Multi-lang/REST — Client-side filtering & sorting
// LR #12: AI Integration — skeleton, empty/error states, role-aware cards

// === State ===
var filterState = {
  search: '',
  minPrice: null,
  maxPrice: null,
  sort: 'newest',
  limit: 9,
  offset: 0,
};

var allServices = [];    // full data from API
var filteredServices = []; // after client-side filtering
var currentPage = 1;
var perPage = 9;

// === DOM refs ===
var grid = document.getElementById('services-grid');
var emptyState = document.getElementById('empty-state');
var errorState = document.getElementById('error-state');
var errorMessage = document.getElementById('error-message');
var paginationEl = document.getElementById('pagination');
var retryBtn = document.getElementById('error-retry');
var sidebarToggle = document.getElementById('sidebar-toggle');
var sidebarClose = document.getElementById('sidebar-close');
var sidebar = document.getElementById('catalog-sidebar');

// === Load services from API ===
async function loadServices() {
  grid.classList.remove('hidden');
  emptyState.classList.add('hidden');
  errorState.classList.add('hidden');
  paginationEl.classList.add('hidden');
  showSkeletons();

  try {
    var response = await apiFetchServices({ limit: 100, offset: 0 });
    var data = response.data || [];
    allServices = data;

    applyClientFilters();
  } catch (err) {
    hideSkeletons();
    if (err.status === 401 || err.status === 403) {
      return;
    }
    showError(err.message || 'Failed to load services');
  }
}

// === Client-side filter + sort + pagination ===
function applyClientFilters() {
  var filtered = allServices.slice();

  // Search filter
  var search = filterState.search.trim().toLowerCase();
  if (search) {
    filtered = filtered.filter(function (s) {
      return (
        (s.title && s.title.toLowerCase().indexOf(search) !== -1) ||
        (s.description && s.description.toLowerCase().indexOf(search) !== -1)
      );
    });
  }

  // Price range
  var minP = parseFloat(filterState.minPrice);
  var maxP = parseFloat(filterState.maxPrice);
  if (!isNaN(minP) && minP > 0) {
    filtered = filtered.filter(function (s) { return parseFloat(s.price) >= minP; });
  }
  if (!isNaN(maxP) && maxP > 0) {
    filtered = filtered.filter(function (s) { return parseFloat(s.price) <= maxP; });
  }

  // Sort
  filtered.sort(function (a, b) {
    switch (filterState.sort) {
      case 'oldest':
        return new Date(a.created_at) - new Date(b.created_at);
      case 'price_asc':
        return parseFloat(a.price) - parseFloat(b.price);
      case 'price_desc':
        return parseFloat(b.price) - parseFloat(a.price);
      case 'title_asc':
        return (a.title || '').localeCompare(b.title || '');
      case 'newest':
      default:
        return new Date(b.created_at) - new Date(a.created_at);
    }
  });

  filteredServices = filtered;
  currentPage = 1;
  filterState.offset = 0;
  renderPage();
}

// === Render current page ===
function renderPage() {
  var start = (currentPage - 1) * perPage;
  var end = start + perPage;
  var pageData = filteredServices.slice(start, end);
  var totalPages = Math.ceil(filteredServices.length / perPage) || 1;

  hideSkeletons();

  if (filteredServices.length === 0) {
    grid.classList.add('hidden');
    emptyState.classList.remove('hidden');
    errorState.classList.add('hidden');
    paginationEl.classList.add('hidden');
    return;
  }

  grid.classList.remove('hidden');
  emptyState.classList.add('hidden');
  errorState.classList.add('hidden');
  renderCards(pageData);
  renderPagination(currentPage, totalPages);
}

// === Render service cards ===
function renderCards(services) {
  var auth = checkAuth();
  var role = auth.role;

  var html = services.map(function (s) {
    var price = parseFloat(s.price);
    var formattedPrice = '$' + price.toFixed(2);

    var priceStr = price % 1 === 0 ? '$' + price.toFixed(2) : formattedPrice;

    // Format date
    var dateStr = '';
    try {
      dateStr = new Date(s.created_at).toLocaleDateString('en-US', {
        year: 'numeric', month: 'short', day: 'numeric',
      });
    } catch (e) {
      dateStr = '';
    }

    var truncatedDesc = s.description || 'No description provided.';
    if (truncatedDesc.length > 150) {
      truncatedDesc = truncatedDesc.substring(0, 147) + '...';
    }

    var statusBadge = renderBadge(s.status);

    // Role-aware order button
    var orderBtn = '';
    if (!auth.isAuthenticated) {
      orderBtn = '<a href="/register" class="btn btn-primary btn-sm">Order</a>';
    } else if (role === 'client') {
      orderBtn =
        '<button class="btn btn-primary btn-sm order-btn" data-service-id="' +
        s.id +
        '" data-provider-id="' +
        s.provider_id +
        '" data-price="' +
        s.price +
        '" data-service-title="' +
        escapeHtml(s.title).replace(/"/g, '&quot;') +
        '">Order</button>';
    } else if (role === 'provider') {
      orderBtn =
        '<button class="btn btn-outline btn-sm" disabled title="Available only for clients">Order</button>';
    } else {
      orderBtn = '<button class="btn btn-primary btn-sm order-btn" data-service-id="' + s.id + '">Order</button>';
    }

    return (
      '<div class="service-card" data-id="' +
      s.id +
      '">' +
      '<div class="service-card-image">' +
      '<div class="service-card-placeholder">&#x1F6D2;</div>' +
      '</div>' +
      '<div class="service-card-body">' +
      '<h3 class="service-card-title">' +
      escapeHtml(s.title) +
      '</h3>' +
      '<p class="service-card-description">' +
      escapeHtml(truncatedDesc) +
      '</p>' +
      '<div class="service-card-meta">' +
      statusBadge +
      '<span class="service-card-category">' +
      escapeHtml(dateStr) +
      '</span>' +
      '</div>' +
      '</div>' +
      '<div class="service-card-footer">' +
      '<span class="service-card-price">' +
      formattedPrice +
      '</span>' +
      orderBtn +
      '</div>' +
      '</div>'
    );
  }).join('');

  grid.innerHTML = html;

  // Attach order button listeners
  grid.querySelectorAll('.order-btn').forEach(function (btn) {
    btn.addEventListener('click', function (e) {
      e.preventDefault();
      var serviceId = btn.getAttribute('data-service-id');
      var providerId = btn.getAttribute('data-provider-id');
      var price = btn.getAttribute('data-price');
      var serviceTitle = btn.getAttribute('data-service-title');
      handleOrderClick(serviceId, providerId, price, serviceTitle);
    });
  });
}

// === Order button handler (opens modal) ===
function handleOrderClick(serviceId, providerId, price, serviceTitle) {
  var auth = checkAuth();
  if (!auth.isAuthenticated) {
    window.location.href = '/register';
    return;
  }
  if (auth.role !== 'client') {
    showToast('Only clients can place orders', 'error');
    return;
  }

  var amount = parseFloat(price);
  if (isNaN(amount) || amount <= 0) {
    showToast('Invalid service price', 'error');
    return;
  }

  openOrderModal(serviceId, providerId, price, serviceTitle);
}

// === Render pagination ===
function renderPagination(current, total) {
  if (total <= 1) {
    paginationEl.classList.add('hidden');
    return;
  }
  paginationEl.classList.remove('hidden');

  var html = '';

  // Prev
  html +=
    '<button class="page-prev" ' +
    (current <= 1 ? 'disabled' : '') +
    ' data-page="' +
    (current - 1) +
    '">&laquo; Prev</button>';

  // Page numbers
  var startPage = Math.max(1, current - 2);
  var endPage = Math.min(total, current + 2);

  if (startPage > 1) {
    html += '<button class="page-num" data-page="1">1</button>';
    if (startPage > 2) {
      html += '<span class="page-ellipsis">&hellip;</span>';
    }
  }

  for (var i = startPage; i <= endPage; i++) {
    html +=
      '<button class="page-num' +
      (i === current ? ' active' : '') +
      '" data-page="' +
      i +
      '">' +
      i +
      '</button>';
  }

  if (endPage < total) {
    if (endPage < total - 1) {
      html += '<span class="page-ellipsis">&hellip;</span>';
    }
    html += '<button class="page-num" data-page="' + total + '">' + total + '</button>';
  }

  // Next
  html +=
    '<button class="page-next" ' +
    (current >= total ? 'disabled' : '') +
    ' data-page="' +
    (current + 1) +
    '">Next &raquo;</button>';

  paginationEl.innerHTML = html;

  // Bind click events
  paginationEl.querySelectorAll('button').forEach(function (btn) {
    btn.addEventListener('click', function () {
      var page = parseInt(btn.getAttribute('data-page'), 10);
      if (isNaN(page) || page < 1 || page > total) return;
      currentPage = page;
      renderPage();
      window.scrollTo({ top: 0, behavior: 'smooth' });
    });
  });
}

// === Skeleton helpers ===
function showSkeletons() {
  // Skeletons are in the HTML already, just make sure grid shows them
  grid.innerHTML =
    '<div class="skeleton-card"><div class="skeleton skeleton-image"></div><div class="skeleton skeleton-title"></div><div class="skeleton skeleton-text"></div><div class="skeleton skeleton-price"></div></div>' +
    '<div class="skeleton-card"><div class="skeleton skeleton-image"></div><div class="skeleton skeleton-title"></div><div class="skeleton skeleton-text"></div><div class="skeleton skeleton-price"></div></div>' +
    '<div class="skeleton-card"><div class="skeleton skeleton-image"></div><div class="skeleton skeleton-title"></div><div class="skeleton skeleton-text"></div><div class="skeleton skeleton-price"></div></div>';
}

function hideSkeletons() {
  // Skeletons will be replaced when renderCards is called
}

// === Error state ===
function showError(msg) {
  grid.classList.add('hidden');
  emptyState.classList.add('hidden');
  errorState.classList.remove('hidden');
  paginationEl.classList.add('hidden');
  errorMessage.textContent = msg || 'Failed to load services. Please try again.';
}

// === Apply filters from UI ===
function applyFilters() {
  var search = document.getElementById('filter-search').value;
  var minPrice = document.getElementById('filter-price-min').value;
  var maxPrice = document.getElementById('filter-price-max').value;
  var sort = document.getElementById('filter-sort').value;

  // Validate price
  var minP = minPrice ? parseFloat(minPrice) : null;
  var maxP = maxPrice ? parseFloat(maxPrice) : null;
  if (minP !== null && maxP !== null && minP > maxP) {
    showToast('Min price cannot be greater than max price', 'error');
    return;
  }

  filterState.search = search;
  filterState.minPrice = minP;
  filterState.maxPrice = maxP;
  filterState.sort = sort;

  // Update URL
  var params = new URLSearchParams();
  if (search) params.set('search', search);
  if (minP) params.set('min_price', minP);
  if (maxP) params.set('max_price', maxP);
  if (sort !== 'newest') params.set('sort', sort);
  var qs = params.toString();
  var url = '/catalog' + (qs ? '?' + qs : '');
  history.pushState(null, '', url);

  applyClientFilters();
}

// === Reset filters ===
function resetFilters() {
  document.getElementById('filter-search').value = '';
  document.getElementById('filter-price-min').value = '';
  document.getElementById('filter-price-max').value = '';
  document.getElementById('filter-sort').value = 'newest';

  filterState.search = '';
  filterState.minPrice = null;
  filterState.maxPrice = null;
  filterState.sort = 'newest';

  history.pushState(null, '', '/catalog');

  applyClientFilters();
}

// === Read filters from URL on load ===
function readFiltersFromUrl() {
  var params = new URLSearchParams(window.location.search);
  var search = params.get('search') || '';
  var minPrice = params.get('min_price') || '';
  var maxPrice = params.get('max_price') || '';
  var sort = params.get('sort') || 'newest';

  document.getElementById('filter-search').value = search;
  document.getElementById('filter-price-min').value = minPrice;
  document.getElementById('filter-price-max').value = maxPrice;
  document.getElementById('filter-sort').value = sort;

  filterState.search = search;
  filterState.minPrice = minPrice ? parseFloat(minPrice) : null;
  filterState.maxPrice = maxPrice ? parseFloat(maxPrice) : null;
  filterState.sort = sort;
}

// === Debounced search ===
var debouncedSearch = debounce(function () {
  applyFilters();
}, 300);

// === Init catalog page ===
function initCatalogPage() {
  readFiltersFromUrl();

  // Event listeners
  document.getElementById('filter-apply').addEventListener('click', applyFilters);
  document.getElementById('filter-reset').addEventListener('click', resetFilters);

  document.getElementById('filter-search').addEventListener('input', debouncedSearch);

  document.getElementById('filter-price-min').addEventListener('change', applyFilters);
  document.getElementById('filter-price-max').addEventListener('change', applyFilters);
  document.getElementById('filter-sort').addEventListener('change', applyFilters);

  // Retry button
  if (retryBtn) {
    retryBtn.addEventListener('click', loadServices);
  }

  // Sidebar toggle (mobile)
  if (sidebarToggle) {
    sidebarToggle.addEventListener('click', function () {
      sidebar.classList.toggle('open');
    });
  }
  if (sidebarClose) {
    sidebarClose.addEventListener('click', function () {
      sidebar.classList.remove('open');
    });
  }

  // Load data
  loadServices();
}

// Run on DOMContentLoaded if on catalog page
document.addEventListener('DOMContentLoaded', function () {
  if (window.location.pathname === '/catalog') {
    initCatalogPage();
  }
});

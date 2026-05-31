// LR #6: Web/DB — Authentication module for Service Marketplace
// LR #10: Multi-lang/REST — JWT-based auth with role-based redirect
// LR #12: AI Integration — login, register, logout, navbar update

// === Save auth data to localStorage ===
function saveAuthData(tokens, user) {
  setToken(tokens.access_token, tokens.refresh_token);
  setUser(user);
}

// === Check current auth state ===
function checkAuth() {
  var token = getToken();
  var user = getUser();
  if (!token || !user) {
    return { isAuthenticated: false, user: null, role: null };
  }
  return { isAuthenticated: true, user: user, role: user.role };
}

// === Handle login form submission ===
async function handleLogin(event) {
  event.preventDefault();
  if (window._isProcessing) return;
  window._isProcessing = true;

  var form = event.target;
  var submitBtn = form.querySelector('button[type="submit"]');
  if (submitBtn) submitBtn.disabled = true;

  clearAlerts();
  form.querySelectorAll('.form-error').forEach(function (el) { el.remove(); });

  var email = form.querySelector('#email');
  var password = form.querySelector('#password');

  // Client-side validation
  var hasError = false;
  if (!email.value.trim()) {
    showFieldError(email, 'Email is required');
    hasError = true;
  }
  if (!password.value || password.value.length < 8) {
    showFieldError(password, 'Password must be at least 8 characters');
    hasError = true;
  }
  if (hasError) {
    if (submitBtn) submitBtn.disabled = false;
    window._isProcessing = false;
    return;
  }

  try {
    var response = await apiLogin(email.value.trim(), password.value);
    var data = response.data;

    // LR #12: Save tokens and user, then redirect by role
    saveAuthData(
      {
        access_token: data.access_token,
        refresh_token: data.refresh_token,
      },
      data.user
    );

    showToast('Login successful!', 'success');

    // ✅ FIX: Redirect based on role, NEVER to /
    var role = data.user.role || 'client';
    if (role === 'provider') {
      setTimeout(function () {
        window.location.href = '/dashboard/executor';
      }, 300);
    } else {
      setTimeout(function () {
        window.location.href = '/dashboard/client';
      }, 300);
    }
  } catch (err) {
    showAlert(err.message, 'error');
    if (submitBtn) submitBtn.disabled = false;
    window._isProcessing = false;
  }
}

// === Handle register form submission ===
async function handleRegister(event) {
  event.preventDefault();
  if (window._isProcessing) return;
  window._isProcessing = true;

  var form = event.target;
  var submitBtn = form.querySelector('button[type="submit"]');
  if (submitBtn) submitBtn.disabled = true;

  clearAlerts();
  form.querySelectorAll('.form-error').forEach(function (el) { el.remove(); });

  var email = form.querySelector('#email');
  var password = form.querySelector('#password');
  var passwordConfirm = form.querySelector('#password-confirm');
  var role = form.querySelector('#role');

  // Client-side validation
  var hasError = false;
  if (!email.value.trim()) {
    showFieldError(email, 'Email is required');
    hasError = true;
  }
  if (!password.value || password.value.length < 8) {
    showFieldError(password, 'Password must be at least 8 characters');
    hasError = true;
  }
  if (password.value !== passwordConfirm.value) {
    showFieldError(passwordConfirm, 'Passwords do not match');
    hasError = true;
  }
  if (hasError) {
    if (submitBtn) submitBtn.disabled = false;
    window._isProcessing = false;
    return;
  }

  try {
    var response = await apiRegister(
      email.value.trim(),
      password.value,
      role.value
    );
    var data = response.data;

    // LR #12: Save tokens and user, then redirect by role
    saveAuthData(
      {
        access_token: data.access_token,
        refresh_token: data.refresh_token,
      },
      data.user
    );

    showToast('Registration successful!', 'success');

    // ✅ FIX: Redirect based on role, NEVER to /
    var userRole = data.user.role || 'client';
    if (userRole === 'provider') {
      setTimeout(function () {
        window.location.href = '/dashboard/executor';
      }, 300);
    } else {
      setTimeout(function () {
        window.location.href = '/dashboard/client';
      }, 300);
    }
  } catch (err) {
    showAlert(err.message, 'error');
    if (submitBtn) submitBtn.disabled = false;
    window._isProcessing = false;
  }
}

// === Handle logout ===
function handleLogout(event) {
  if (event) event.preventDefault();
  clearTokens();
  showToast('You have been logged out', 'info');
  setTimeout(function () {
    window.location.href = '/';
  }, 300);
}

// === Update navbar based on auth state ===
function updateNavbar() {
  console.log('updateNavbar called, auth:', checkAuth());
  var nav = document.getElementById('navbar');
  if (!nav) return;

  var menu = document.getElementById('navbar-menu');
  var userSection = document.getElementById('navbar-user');
  var emailEl = document.getElementById('user-email');
  var roleEl = document.getElementById('user-role');

  var auth = checkAuth();
  var user = auth.user;

  if (auth.isAuthenticated && user) {
    var roleLabel =
      user.role === 'provider'
        ? 'Executor'
        : user.role === 'admin'
        ? 'Admin'
        : 'Client';

    var dashUrl = '/dashboard/' + (user.role === 'provider' ? 'executor' : 'client');

    menu.innerHTML =
      '<a href="/catalog">Catalog</a>' +
      '<a href="/wallet">Wallet</a>' +
      '<a href="' + dashUrl + '">Dashboard</a>';

    emailEl.textContent = user.email;
    roleEl.textContent = roleLabel;
    userSection.classList.remove('hidden');

    document.getElementById('logout-btn').onclick = handleLogout;
  } else {
    menu.innerHTML =
      '<a href="/login">Login</a>' +
      '<a href="/register">Register</a>';
    userSection.classList.add('hidden');
  }
}

// === Show field error ===
function showFieldError(input, msg) {
  var err = document.createElement('div');
  err.className = 'form-error';
  err.textContent = msg;
  input.parentNode.appendChild(err);
  input.style.borderColor = 'var(--danger)';
  input.addEventListener(
    'input',
    function () {
      input.style.borderColor = '';
      err.remove();
    },
    { once: true }
  );
}

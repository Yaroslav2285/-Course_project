// LR #6: Web/DB — Main entry point for Service Marketplace UI
// LR #12: AI Integration — page-specific initialization

document.addEventListener('DOMContentLoaded', function () {
  // Always render navbar
  updateNavbar();

  // Page-specific initialization based on body data attribute or path
  var path = window.location.pathname;

  if (path === '/') {
    var auth = checkAuth();
    if (auth.isAuthenticated) {
      var dest = auth.role === 'provider' ? '/dashboard/executor' : '/dashboard/client';
      window.location.href = dest;
      return;
    }
  } else if (path === '/login') {
    initLoginPage();
  } else if (path === '/register') {
    initRegisterPage();
  }
});

function initLoginPage() {
  var form = document.getElementById('login-form');
  if (!form) return;

  // If already logged in, redirect to dashboard
  var auth = checkAuth();
  if (auth.isAuthenticated) {
    var dest =
      auth.role === 'provider'
        ? '/dashboard/executor'
        : '/dashboard/client';
    window.location.href = dest;
    return;
  }

  // Remove existing listeners by cloning
  form.addEventListener('submit', handleLogin);
}

function initRegisterPage() {
  var form = document.getElementById('register-form');
  if (!form) return;

  // If already logged in, redirect to dashboard
  var auth = checkAuth();
  if (auth.isAuthenticated) {
    var dest =
      auth.role === 'provider'
        ? '/dashboard/executor'
        : '/dashboard/client';
    window.location.href = dest;
    return;
  }

  form.addEventListener('submit', handleRegister);
}

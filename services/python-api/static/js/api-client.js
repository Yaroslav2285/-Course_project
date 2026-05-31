// LR #6: Web/DB — Enhanced fetch wrapper with retry + timeout (Phase 7.5)
// LR #10: Multi-lang/REST — X-Request-ID, 401 redirect, retry logic, timeout
// LR #12: AI Integration — retry with exponential backoff
// LR #15: Security/UX — timeout 8s, exponential backoff, 401→/login

async function apiClient(path, options, retries) {
  retries = retries !== undefined ? retries : 2;
  options = options || {};
  options.headers = options.headers || {};

  if (!options.headers['X-Request-ID']) {
    options.headers['X-Request-ID'] = crypto.randomUUID();
  }

  var timeoutMs = 8000;

  for (var attempt = 0; attempt <= retries; attempt++) {
    try {
      var controller = new AbortController();
      var timeoutId = setTimeout(function () {
        controller.abort();
      }, timeoutMs);

      options.signal = controller.signal;

      var result = await apiFetch(path, options);
      clearTimeout(timeoutId);
      return result;
    } catch (err) {
      var isLastAttempt = attempt >= retries;
      var isTimeout = err && (err.name === 'AbortError' || err.code === 20);
      var isServerError = (err.status >= 500 && err.status <= 599) || err.status === 0;
      var is429 = err.status === 429;
      var isRetryable = is429 || isServerError || isTimeout;

      if (isLastAttempt || !isRetryable) {
        throw err;
      }

      var delay = Math.pow(2, attempt) * 1000;
      await new Promise(function (resolve) {
        setTimeout(resolve, delay);
      });
    }
  }
}

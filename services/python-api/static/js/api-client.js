// LR #6: Web/DB — Enhanced fetch wrapper with retry (Phase 7.4)
// LR #10: Multi-lang/REST — X-Request-ID, 401 redirect, retry logic
// LR #12: AI Integration — retry with exponential backoff

async function apiClient(path, options, retries) {
  retries = retries !== undefined ? retries : 1;
  options = options || {};
  options.headers = options.headers || {};

  if (!options.headers['X-Request-ID']) {
    options.headers['X-Request-ID'] = crypto.randomUUID();
  }

  for (var attempt = 0; attempt <= retries; attempt++) {
    try {
      var result = await apiFetch(path, options);
      return result;
    } catch (err) {
      var isLastAttempt = attempt >= retries;
      var isServerError = err.status >= 500 || err.status === 0;
      var isRetryable = err.status === 429 || isServerError;

      if (isLastAttempt || !isRetryable) {
        throw err;
      }

      await new Promise(function (resolve) {
        setTimeout(resolve, 1000 * (attempt + 1));
      });
    }
  }
}

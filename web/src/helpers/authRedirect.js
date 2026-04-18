const DEFAULT_POST_LOGIN_REDIRECT = '/console';
const POST_LOGIN_REDIRECT_KEY = 'post_login_redirect';
const AUTH_ROUTE_PREFIXES = ['/login', '/register', '/reset', '/oauth'];

function getSessionStorage() {
  if (typeof window === 'undefined') {
    return null;
  }

  try {
    return window.sessionStorage;
  } catch (error) {
    return null;
  }
}

export function buildPathFromLocation(locationLike) {
  if (!locationLike) {
    return '';
  }

  if (typeof locationLike === 'string') {
    return locationLike;
  }

  const pathname = locationLike.pathname || '';
  const search = locationLike.search || '';
  const hash = locationLike.hash || '';
  return `${pathname}${search}${hash}`;
}

export function normalizeRedirectTarget(
  target,
  fallback = DEFAULT_POST_LOGIN_REDIRECT,
) {
  let nextTarget = buildPathFromLocation(target);

  if (!nextTarget) {
    return fallback;
  }

  if (/^https?:\/\//i.test(nextTarget)) {
    if (typeof window === 'undefined') {
      return fallback;
    }

    try {
      const parsed = new URL(nextTarget);
      if (parsed.origin !== window.location.origin) {
        return fallback;
      }
      nextTarget = `${parsed.pathname}${parsed.search}${parsed.hash}`;
    } catch (error) {
      return fallback;
    }
  }

  if (!nextTarget.startsWith('/')) {
    return fallback;
  }

  const isAuthRoute = AUTH_ROUTE_PREFIXES.some((prefix) => {
    return (
      nextTarget === prefix ||
      nextTarget.startsWith(`${prefix}/`) ||
      nextTarget.startsWith(`${prefix}?`) ||
      nextTarget.startsWith(`${prefix}#`)
    );
  });

  if (isAuthRoute) {
    return fallback;
  }

  return nextTarget;
}

export function savePostLoginRedirect(target) {
  const storage = getSessionStorage();
  const normalizedTarget = normalizeRedirectTarget(target, '');

  if (!storage || !normalizedTarget) {
    return '';
  }

  storage.setItem(POST_LOGIN_REDIRECT_KEY, normalizedTarget);
  return normalizedTarget;
}

export function readPostLoginRedirect() {
  const storage = getSessionStorage();
  if (!storage) {
    return '';
  }

  return normalizeRedirectTarget(storage.getItem(POST_LOGIN_REDIRECT_KEY), '');
}

export function clearPostLoginRedirect() {
  const storage = getSessionStorage();
  if (!storage) {
    return;
  }

  storage.removeItem(POST_LOGIN_REDIRECT_KEY);
}

export function buildLoginHref(target, options = {}) {
  const { expired = false } = options;
  const normalizedTarget = normalizeRedirectTarget(target, '');
  const searchParams = new URLSearchParams();

  if (normalizedTarget) {
    searchParams.set('redirect', normalizedTarget);
  }

  if (expired) {
    searchParams.set('expired', 'true');
  }

  const query = searchParams.toString();
  return query ? `/login?${query}` : '/login';
}

export function resolvePostLoginRedirect(options = {}) {
  const {
    location,
    storedRedirect = readPostLoginRedirect(),
    fallback = DEFAULT_POST_LOGIN_REDIRECT,
  } = options;

  const fromState = normalizeRedirectTarget(location?.state?.from, '');
  if (fromState) {
    return fromState;
  }

  const queryRedirect = new URLSearchParams(location?.search || '').get(
    'redirect',
  );
  const fromQuery = normalizeRedirectTarget(queryRedirect, '');
  if (fromQuery) {
    return fromQuery;
  }

  const fromStorage = normalizeRedirectTarget(storedRedirect, '');
  if (fromStorage) {
    return fromStorage;
  }

  return fallback;
}

export function consumePostLoginRedirect(options = {}) {
  const target = resolvePostLoginRedirect(options);
  clearPostLoginRedirect();
  return target;
}

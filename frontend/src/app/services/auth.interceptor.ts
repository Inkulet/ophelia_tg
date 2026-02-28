import { HttpInterceptorFn } from '@angular/common/http';

const AUTH_STORAGE_KEYS = ['ophelia_cms_jwt', 'cms_jwt', 'auth_token', 'token'];

export const cmsAuthInterceptor: HttpInterceptorFn = (req, next) => {
  const token = readAuthToken();
  if (!token || !isCMSRequest(req.url)) {
    return next(req);
  }

  return next(
    req.clone({
      setHeaders: {
        Authorization: `Bearer ${token}`,
      },
    }),
  );
};

function readAuthToken(): string | null {
  if (typeof window === 'undefined' || typeof localStorage === 'undefined') {
    return null;
  }

  for (const key of AUTH_STORAGE_KEYS) {
    const raw = localStorage.getItem(key);
    if (!raw) {
      continue;
    }
    const token = raw.trim();
    if (token !== '') {
      return token;
    }
  }
  return null;
}

function isCMSRequest(rawUrl: string): boolean {
  if ((rawUrl ?? '').trim() === '') {
    return false;
  }

  if (typeof window === 'undefined') {
    return rawUrl.startsWith('/cms/');
  }

  try {
    const url = new URL(rawUrl, window.location.origin);
    return url.pathname.startsWith('/cms/');
  } catch {
    return rawUrl.startsWith('/cms/');
  }
}

import { readEnv, readEnvNumber } from './utils';

export const API_BASE_URL = readEnv(import.meta.env.PUBLIC_API_BASE_URL, '');

const isLoopbackHost = (host: string): boolean => host === 'localhost' || host === '127.0.0.1';

const resolveCheckoutBaseUrl = (): string => {
  const configured = readEnv(import.meta.env.PUBLIC_CHECKOUT_BASE_URL, '');

  if (typeof window !== 'undefined' && !isLoopbackHost(window.location.hostname)) {
    return `${window.location.protocol}//${window.location.hostname}:9090`;
  }

  return configured || 'http://localhost:9090';
};

export const CHECKOUT_BASE_URL = resolveCheckoutBaseUrl();

export const USER_ID_STORAGE_KEY = readEnv(
  import.meta.env.PUBLIC_USER_ID_STORAGE_KEY,
  'queue-service:user-id',
);

export const USER_ROLE_STORAGE_KEY = readEnv(
  import.meta.env.PUBLIC_USER_ROLE_STORAGE_KEY,
  'queue-service:user-role',
);

export const APP_BASENAME = '/avito';

export const APP_STALE_TIME = readEnvNumber(import.meta.env.PUBLIC_APP_STALE_TIME, 30000);

export const appPath = (path = '/'): string => {
  if (path === '/') return APP_BASENAME;

  return `${APP_BASENAME}${path.startsWith('/') ? path : `/${path}`}`;
};

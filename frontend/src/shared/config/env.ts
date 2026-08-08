import { readEnv, readEnvNumber } from './utils';

export const API_BASE_URL = readEnv(import.meta.env.PUBLIC_API_BASE_URL, '');

export const CHECKOUT_BASE_URL = readEnv(
  import.meta.env.PUBLIC_CHECKOUT_BASE_URL,
  'http://localhost:9090',
);

export const USER_ID_STORAGE_KEY = readEnv(
  import.meta.env.PUBLIC_USER_ID_STORAGE_KEY,
  'queue-service:user-id',
);

export const APP_BASENAME = '/avito';

export const APP_STALE_TIME = readEnvNumber(import.meta.env.PUBLIC_APP_STALE_TIME, 30000);

/** Absolute in-app path under `APP_BASENAME` (e.g. `/` → `/avito`, `/order-info/1` → `/avito/order-info/1`). */
export const appPath = (path = '/'): string => {
  if (path === '/') return APP_BASENAME;

  return `${APP_BASENAME}${path.startsWith('/') ? path : `/${path}`}`;
};

import type { Nullable } from '@shared/model';

export const secondsToExpiry = (expiresAt?: string): Nullable<number> => {
  if (!expiresAt) return null;

  return Math.max(0, Math.ceil((new Date(expiresAt).getTime() - Date.now()) / 1000));
};

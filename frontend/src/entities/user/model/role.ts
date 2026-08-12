import { USER_ROLE_STORAGE_KEY } from '@shared/config';

export const USER_ROLES = ['buyer', 'seller'] as const;

export type UserRole = (typeof USER_ROLES)[number];

export const isUserRole = (value: string | null): value is UserRole =>
  value === 'buyer' || value === 'seller';

export const readStoredRole = (): UserRole => {
  const stored = localStorage.getItem(USER_ROLE_STORAGE_KEY);
  return isUserRole(stored) ? stored : 'buyer';
};

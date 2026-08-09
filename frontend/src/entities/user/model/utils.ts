import { USER_ID_STORAGE_KEY } from '@shared/config';

/**
 * костыль для создания uuid
 * что бы не привязывать https к сайту
 */
const createUuid = () => {
  if (typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID();
  }

  const bytes = new Uint8Array(16);
  crypto.getRandomValues(bytes);
  bytes[6] = (bytes[6] & 0x0f) | 0x40;
  bytes[8] = (bytes[8] & 0x3f) | 0x80;
  const hex = [...bytes].map((b) => b.toString(16).padStart(2, '0')).join('');
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
};

export const createUserId = () => {
  const stored = localStorage.getItem(USER_ID_STORAGE_KEY);
  if (stored) return stored;

  const userId = createUuid();
  localStorage.setItem(USER_ID_STORAGE_KEY, userId);
  return userId;
};

import { API_BASE_URL } from '@shared/config';

export const getWsUrl = (productId: string, userId: string) => {
  const apiUrl = new URL(API_BASE_URL || '/api/v1', window.location.origin);
  apiUrl.protocol = apiUrl.protocol === 'https:' ? 'wss:' : 'ws:';
  apiUrl.pathname = `${apiUrl.pathname.replace(/\/$/, '')}/queue/${productId}/members/me`;
  apiUrl.searchParams.set('user_id', userId);
  return apiUrl.toString();
};

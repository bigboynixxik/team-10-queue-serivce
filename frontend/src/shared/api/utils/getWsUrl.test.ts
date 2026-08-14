import { afterEach, describe, expect, rs, test } from '@rstest/core';

rs.mock('@shared/config', () => ({
  API_BASE_URL: 'https://api.example.com/api/v1',
}));

const { getWsUrl } = await import('./getWsUrl');

describe('getWsUrl', () => {
  afterEach(() => {
    rs.unstubAllGlobals();
  });

  test('builds wss url with membership path and user_id', () => {
    rs.stubGlobal('location', new URL('https://app.example.com/avito'));

    const url = new URL(getWsUrl('product-1', 'user-42'));

    expect(url.protocol).toBe('wss:');
    expect(url.host).toBe('api.example.com');
    expect(url.pathname).toBe('/api/v1/queue/product-1/members/me');
    expect(url.searchParams.get('user_id')).toBe('user-42');
  });
});

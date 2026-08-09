import { describe, expect, rs, test } from '@rstest/core';

rs.mock('@shared/config', () => ({
  CHECKOUT_BASE_URL: 'https://checkout.example',
}));

const { buildCheckoutUrl } = await import('./buildCheckoutUrl');

describe('buildCheckoutUrl', () => {
  test('builds checkout url with token and product_id', () => {
    const url = new URL(buildCheckoutUrl('product-1', 'token-abc'));

    expect(url.origin).toBe('https://checkout.example');
    expect(url.pathname).toBe('/checkout');
    expect(url.searchParams.get('token')).toBe('token-abc');
    expect(url.searchParams.get('product_id')).toBe('product-1');
  });
});

import { CHECKOUT_BASE_URL } from '@shared/config';

export const buildCheckoutUrl = (productId: string, token: string): string => {
  const checkoutUrl = new URL('/checkout', CHECKOUT_BASE_URL);

  checkoutUrl.searchParams.set('token', token);
  checkoutUrl.searchParams.set('product_id', productId);

  return checkoutUrl.toString();
};

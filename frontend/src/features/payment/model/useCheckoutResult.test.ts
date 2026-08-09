import { afterEach, beforeEach, describe, expect, rs, test } from '@rstest/core';
import type { QueryClient } from '@tanstack/react-query';

import { queueQueries } from '@entities/queue';
import { createTestQueryClient, renderHookWithProviders } from '@test/render';

rs.mock('@shared/config', () => ({
  CHECKOUT_BASE_URL: 'https://checkout.example',
}));

const { useCheckoutResult } = await import('./useCheckoutResult');

const dispatchMessage = (origin: string, data: unknown) => {
  window.dispatchEvent(new MessageEvent('message', { origin, data }));
};

describe('useCheckoutResult', () => {
  let queryClient: QueryClient;
  let invalidateSpy: ReturnType<typeof rs.spyOn>;

  beforeEach(() => {
    queryClient = createTestQueryClient();
    invalidateSpy = rs.spyOn(queryClient, 'invalidateQueries');
  });

  afterEach(() => {
    invalidateSpy.mockRestore();
  });

  test('ignores messages from another origin', () => {
    renderHookWithProviders(() => useCheckoutResult('product-1'), { queryClient });

    dispatchMessage('https://evil.example', {
      source: 'avito-checkout',
      event: 'payment_succeeded',
    });

    expect(invalidateSpy).not.toHaveBeenCalled();
  });

  test('ignores wrong payload shape', () => {
    renderHookWithProviders(() => useCheckoutResult('product-1'), { queryClient });

    dispatchMessage('https://checkout.example', {
      source: 'avito-checkout',
      event: 'payment_failed',
    });

    expect(invalidateSpy).not.toHaveBeenCalled();
  });

  test('invalidates membership query on successful payment message', () => {
    renderHookWithProviders(() => useCheckoutResult('product-1'), { queryClient });

    dispatchMessage('https://checkout.example', {
      source: 'avito-checkout',
      event: 'payment_succeeded',
    });

    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: queueQueries.me('product-1').queryKey,
    });
  });

  test('removes listener on unmount', () => {
    const { unmount } = renderHookWithProviders(() => useCheckoutResult('product-1'), {
      queryClient,
    });

    unmount();

    dispatchMessage('https://checkout.example', {
      source: 'avito-checkout',
      event: 'payment_succeeded',
    });

    expect(invalidateSpy).not.toHaveBeenCalled();
  });
});

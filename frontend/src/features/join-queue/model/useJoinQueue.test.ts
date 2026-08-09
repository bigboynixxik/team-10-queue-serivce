import { beforeEach, describe, expect, rs, test } from '@rstest/core';
import * as reactQuery from '@tanstack/react-query' with { rstest: 'importActual' };
import { act, renderHook } from '@testing-library/react';

import type { Product } from '@entities/product';

const mutate = rs.fn();
const notifyError = rs.fn();

rs.mock('@shared/lib', () => ({
  useErrorNotifier: () => notifyError,
}));

rs.mock('@tanstack/react-query', () => ({
  ...reactQuery,
  useMutation: () => ({ mutate, isPending: false }),
}));

const { useJoinQueue } = await import('./useJoinQueue');

const product: Product = {
  id: 'product-1',
  title: 'Phone',
  description: 'desc',
  price: 1000,
  image: 'https://example.com/phone.png',
};

describe('useJoinQueue', () => {
  beforeEach(() => {
    mutate.mockReset();
    notifyError.mockReset();
  });

  test('joins with quantity payload', () => {
    const { result } = renderHook(() => useJoinQueue(product));

    act(() => {
      result.current.join(2);
    });

    expect(mutate).toHaveBeenCalledWith({ quantity: 2 }, expect.any(Object));
  });

  test('notifies error with join fallback message', () => {
    const error = new Error('fail');
    mutate.mockImplementation(
      (_payload: unknown, options?: { onError?: (error: Error) => void }) => {
        options?.onError?.(error);
      },
    );

    const { result } = renderHook(() => useJoinQueue(product));

    act(() => {
      result.current.join(1);
    });

    expect(notifyError).toHaveBeenCalledWith(error, 'Не удалось встать в очередь');
  });
});

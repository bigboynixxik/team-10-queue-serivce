import { beforeEach, describe, expect, rs, test } from '@rstest/core';
import * as reactQuery from '@tanstack/react-query' with { rstest: 'importActual' };
import { act, renderHook } from '@testing-library/react';

import type { Product } from '@entities/product';

const join = rs.fn();
const toastError = rs.fn();
let stats: { product_count: number } | undefined;

rs.mock('./useJoinQueue', () => ({
  useJoinQueue: () => ({ join, isPending: false }),
}));

rs.mock('@tanstack/react-query', () => ({
  ...reactQuery,
  useQuery: () => ({ data: stats }),
}));

rs.mock('@ui', () => ({
  useToast: () => ({ error: toastError, info: rs.fn(), success: rs.fn() }),
}));

const { useJoinQueueForm } = await import('./useJoinQueueForm');

const product: Product = {
  id: 'product-1',
  title: 'Phone',
  description: 'desc',
  price: 1000,
  image: 'https://example.com/phone.png',
};

describe('useJoinQueueForm', () => {
  beforeEach(() => {
    join.mockReset();
    toastError.mockReset();
    stats = { product_count: 3 };
  });

  test('does not decrease below minimum quantity', () => {
    const { result } = renderHook(() => useJoinQueueForm(product));

    act(() => {
      result.current.decrease();
    });

    expect(result.current.quantity).toBe(1);
  });

  test('increases quantity within stock limit', () => {
    const { result } = renderHook(() => useJoinQueueForm(product));

    act(() => {
      result.current.increase();
    });

    expect(result.current.quantity).toBe(2);
    expect(toastError).not.toHaveBeenCalled();
  });

  test('blocks increase at product_count and shows toast', () => {
    stats = { product_count: 1 };
    const { result } = renderHook(() => useJoinQueueForm(product));

    act(() => {
      result.current.increase();
    });

    expect(result.current.quantity).toBe(1);
    expect(toastError).toHaveBeenCalledWith('Столько товаров купить нельзя. Максимум — 1 шт.');
  });

  test('shows out-of-stock toast when max is 0', () => {
    stats = { product_count: 0 };
    const { result } = renderHook(() => useJoinQueueForm(product));

    act(() => {
      result.current.increase();
    });

    expect(toastError).toHaveBeenCalledWith('Товара больше нет в наличии.');
  });

  test('submits current quantity', () => {
    const { result } = renderHook(() => useJoinQueueForm(product));

    act(() => {
      result.current.increase();
    });

    act(() => {
      result.current.submit();
    });

    expect(join).toHaveBeenCalledWith(2);
  });

  test('blocks submit when quantity exceeds stock', () => {
    stats = { product_count: 0 };
    const { result } = renderHook(() => useJoinQueueForm(product));

    act(() => {
      result.current.submit();
    });

    expect(join).not.toHaveBeenCalled();
    expect(toastError).toHaveBeenCalledWith('Товара больше нет в наличии.');
  });
});

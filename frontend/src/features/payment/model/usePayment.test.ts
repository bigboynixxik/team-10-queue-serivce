import { beforeEach, describe, expect, rs, test } from '@rstest/core';
import * as reactQuery from '@tanstack/react-query' with { rstest: 'importActual' };
import { act, renderHook } from '@testing-library/react';

const mutate = rs.fn();
const notifyError = rs.fn();
const open = rs.fn();

rs.mock('@shared/lib', () => ({
  useErrorNotifier: () => notifyError,
}));

rs.mock('@tanstack/react-query', () => ({
  ...reactQuery,
  useMutation: () => ({ mutate, isPending: false }),
}));

rs.mock('@shared/config', () => ({
  CHECKOUT_BASE_URL: 'https://checkout.example',
}));

const { usePayment } = await import('./usePayment');

describe('usePayment', () => {
  beforeEach(() => {
    mutate.mockReset();
    notifyError.mockReset();
    open.mockReset();
    rs.stubGlobal('open', open);
  });

  test('notifies error and skips mutate when token is missing', () => {
    const { result } = renderHook(() => usePayment('product-1'));

    act(() => {
      result.current.pay();
    });

    expect(notifyError).toHaveBeenCalledWith(expect.any(Error));
    expect(mutate).not.toHaveBeenCalled();
    expect(open).not.toHaveBeenCalled();
  });

  test('opens checkout window on successful validation', () => {
    mutate.mockImplementation((_token: string, options?: { onSuccess?: () => void }) => {
      options?.onSuccess?.();
    });

    const { result } = renderHook(() => usePayment('product-1', 'token-1'));

    act(() => {
      result.current.pay();
    });

    expect(mutate).toHaveBeenCalledWith('token-1', expect.any(Object));
    expect(open).toHaveBeenCalledWith(
      'https://checkout.example/checkout?token=token-1&product_id=product-1',
      'avito-checkout',
    );
  });

  test('notifies error when validation fails', () => {
    const error = new Error('invalid');
    mutate.mockImplementation((_token: string, options?: { onError?: (error: Error) => void }) => {
      options?.onError?.(error);
    });

    const { result } = renderHook(() => usePayment('product-1', 'token-1'));

    act(() => {
      result.current.pay();
    });

    expect(notifyError).toHaveBeenCalledWith(error, 'Не удалось перейти к оплате');
    expect(open).not.toHaveBeenCalled();
  });
});

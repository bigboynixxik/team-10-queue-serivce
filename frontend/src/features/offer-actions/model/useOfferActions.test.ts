import { beforeEach, describe, expect, rs, test } from '@rstest/core';
import * as reactQuery from '@tanstack/react-query' with { rstest: 'importActual' };
import { act, renderHook } from '@testing-library/react';

const acceptMutate = rs.fn();
const declineMutate = rs.fn();
const notifyError = rs.fn();
const useMutation = rs.fn();

rs.mock('@shared/lib', () => ({
  useErrorNotifier: () => notifyError,
}));

rs.mock('@tanstack/react-query', () => ({
  ...reactQuery,
  useMutation,
}));

const { useOfferActions } = await import('./useOfferActions');

describe('useOfferActions', () => {
  beforeEach(() => {
    acceptMutate.mockReset();
    declineMutate.mockReset();
    notifyError.mockReset();
    useMutation.mockReset();
    useMutation
      .mockReturnValueOnce({ mutate: acceptMutate, isPending: false })
      .mockReturnValueOnce({ mutate: declineMutate, isPending: false });
  });

  test('accepts offer with payload', () => {
    const { result } = renderHook(() => useOfferActions('product-1'));

    act(() => {
      result.current.accept({ quantity: 1 });
    });

    expect(acceptMutate).toHaveBeenCalledWith({ quantity: 1 }, expect.any(Object));
  });

  test('declines offer', () => {
    const { result } = renderHook(() => useOfferActions('product-1'));

    act(() => {
      result.current.decline();
    });

    expect(declineMutate).toHaveBeenCalledWith(undefined, expect.any(Object));
  });

  test('notifies accept and decline errors', () => {
    const acceptError = new Error('accept');
    const declineError = new Error('decline');
    acceptMutate.mockImplementation(
      (_payload: unknown, options?: { onError?: (error: Error) => void }) => {
        options?.onError?.(acceptError);
      },
    );
    declineMutate.mockImplementation(
      (_payload: unknown, options?: { onError?: (error: Error) => void }) => {
        options?.onError?.(declineError);
      },
    );

    const { result } = renderHook(() => useOfferActions('product-1'));

    act(() => {
      result.current.accept({ quantity: 1 });
      result.current.decline();
    });

    expect(notifyError).toHaveBeenCalledWith(acceptError, 'Не удалось принять предложение');
    expect(notifyError).toHaveBeenCalledWith(declineError, 'Не удалось отказаться от предложения');
  });

  test('isPending is true when either mutation is pending', () => {
    useMutation.mockReset();
    useMutation
      .mockReturnValueOnce({ mutate: acceptMutate, isPending: true })
      .mockReturnValueOnce({ mutate: declineMutate, isPending: false });

    const { result } = renderHook(() => useOfferActions('product-1'));

    expect(result.current.isPending).toBe(true);
  });
});

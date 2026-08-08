import { beforeEach, describe, expect, rs, test } from '@rstest/core';
import * as reactQuery from '@tanstack/react-query' with { rstest: 'importActual' };
import { act, renderHook } from '@testing-library/react';

const mutate = rs.fn();
const notifyError = rs.fn();

rs.mock('@shared/lib', () => ({
  useErrorNotifier: () => notifyError,
}));

rs.mock('@tanstack/react-query', () => ({
  ...reactQuery,
  useMutation: () => ({ mutate, isPending: true }),
}));

const { useLeaveQueue } = await import('./useLeaveQueue');

describe('useLeaveQueue', () => {
  beforeEach(() => {
    mutate.mockReset();
    notifyError.mockReset();
  });

  test('declines offer and exposes pending state', () => {
    const { result } = renderHook(() => useLeaveQueue('product-1'));

    act(() => {
      result.current.leaveQueue();
    });

    expect(mutate).toHaveBeenCalledWith(undefined, expect.any(Object));
    expect(result.current.isPending).toBe(true);
  });

  test('notifies error with leave fallback message', () => {
    const error = new Error('fail');
    mutate.mockImplementation(
      (_payload: unknown, options?: { onError?: (error: Error) => void }) => {
        options?.onError?.(error);
      },
    );

    const { result } = renderHook(() => useLeaveQueue('product-1'));

    act(() => {
      result.current.leaveQueue();
    });

    expect(notifyError).toHaveBeenCalledWith(error, 'Не удалось выйти из очереди');
  });
});

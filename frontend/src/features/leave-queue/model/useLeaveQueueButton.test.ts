import { beforeEach, describe, expect, rs, test } from '@rstest/core';
import { act, renderHook } from '@testing-library/react';

const leaveQueue = rs.fn();

rs.mock('./useLeaveQueue', () => ({
  useLeaveQueue: () => ({ leaveQueue, isPending: false }),
}));

const { useLeaveQueueButton } = await import('./useLeaveQueueButton');

describe('useLeaveQueueButton', () => {
  beforeEach(() => {
    leaveQueue.mockReset();
  });

  test('opens modal and confirms leave', () => {
    const { result } = renderHook(() => useLeaveQueueButton('product-1'));

    expect(result.current.isOpen).toBe(false);

    act(() => {
      result.current.open();
    });
    expect(result.current.isOpen).toBe(true);

    act(() => {
      result.current.confirmLeave();
    });

    expect(leaveQueue).toHaveBeenCalled();
    expect(result.current.isOpen).toBe(false);
  });

  test('closes without leaving', () => {
    const { result } = renderHook(() => useLeaveQueueButton('product-1'));

    act(() => {
      result.current.open();
      result.current.close();
    });

    expect(result.current.isOpen).toBe(false);
    expect(leaveQueue).not.toHaveBeenCalled();
  });
});

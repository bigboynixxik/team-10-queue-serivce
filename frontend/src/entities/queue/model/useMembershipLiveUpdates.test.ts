import { afterEach, beforeEach, describe, expect, rs, test } from '@rstest/core';
import { act } from '@testing-library/react';

import { createTestQueryClient, renderHookWithProviders } from '@test/render';

type Listeners = {
  onMembership: (membership: { status: string }) => void;
  onError?: () => void;
  onClose?: () => void;
};

const connect = rs.fn();
const disconnect = rs.fn();
let latestListeners: Listeners | undefined;

rs.mock('../api/QueueWs', () => ({
  QueueWs: class {
    connect(listeners: Listeners) {
      latestListeners = listeners;
      connect(listeners);
    }
    disconnect = disconnect;
  },
}));

const { useMembershipLiveUpdates } = await import('./useMembershipLiveUpdates');
const { queueMembershipQueryKey } = await import('./queries');

describe('useMembershipLiveUpdates', () => {
  beforeEach(() => {
    connect.mockReset();
    disconnect.mockReset();
    latestListeners = undefined;
    rs.useFakeTimers();
  });

  afterEach(() => {
    rs.useRealTimers();
  });

  test('does not connect without productId, userId or status', () => {
    renderHookWithProviders(() => useMembershipLiveUpdates('', 'user-1', 'QUEUED'));
    renderHookWithProviders(() => useMembershipLiveUpdates('p1', '', 'QUEUED'));
    renderHookWithProviders(() => useMembershipLiveUpdates('p1', 'user-1'));

    expect(connect).not.toHaveBeenCalled();
  });

  test('does not connect when membership is terminal', () => {
    const queryClient = createTestQueryClient();
    queryClient.setQueryData(queueMembershipQueryKey('p1'), { status: 'PURCHASED' });

    renderHookWithProviders(() => useMembershipLiveUpdates('p1', 'user-1', 'PURCHASED'), {
      queryClient,
    });

    expect(connect).not.toHaveBeenCalled();
  });

  test('writes membership updates into cache', () => {
    const queryClient = createTestQueryClient();

    renderHookWithProviders(() => useMembershipLiveUpdates('p1', 'user-1', 'QUEUED'), {
      queryClient,
    });

    act(() => {
      latestListeners?.onMembership({ status: 'QUEUED' });
    });

    expect(queryClient.getQueryData(queueMembershipQueryKey('p1'))).toEqual({ status: 'QUEUED' });
  });

  test('invalidates on error and reconnects after close', async () => {
    const queryClient = createTestQueryClient();
    const invalidateSpy = rs.spyOn(queryClient, 'invalidateQueries');

    renderHookWithProviders(() => useMembershipLiveUpdates('p1', 'user-1', 'QUEUED'), {
      queryClient,
    });

    expect(connect).toHaveBeenCalledTimes(1);

    act(() => {
      latestListeners?.onError?.();
    });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: queueMembershipQueryKey('p1') });

    act(() => {
      latestListeners?.onClose?.();
    });

    await act(async () => {
      rs.advanceTimersByTime(1000);
    });

    expect(connect).toHaveBeenCalledTimes(2);
  });

  test('disconnects on unmount', () => {
    const { unmount } = renderHookWithProviders(() =>
      useMembershipLiveUpdates('p1', 'user-1', 'QUEUED'),
    );

    unmount();

    expect(disconnect).toHaveBeenCalled();
  });
});

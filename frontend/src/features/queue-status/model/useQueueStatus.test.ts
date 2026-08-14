import { afterEach, beforeEach, describe, expect, rs, test } from '@rstest/core';
import * as reactQuery from '@tanstack/react-query' with { rstest: 'importActual' };
import { act, renderHook } from '@testing-library/react';

import type { Membership } from '@entities/queue';

let membership: Membership | undefined;

rs.mock('@entities/user', () => ({
  useUserStore: {
    use: {
      userId: () => 'user-1',
    },
  },
}));

rs.mock('@entities/queue', () => ({
  useMembershipLiveUpdates: rs.fn(),
  queueQueries: {
    me: (productId: string) => ({
      queryKey: ['queue', 'membership', productId],
    }),
  },
}));

rs.mock('@tanstack/react-query', () => ({
  ...reactQuery,
  useQuery: () => ({
    data: membership,
    isPending: false,
    isError: false,
  }),
}));

const { useQueueStatus } = await import('./useQueueStatus');

describe('useQueueStatus', () => {
  beforeEach(() => {
    membership = undefined;
    rs.useFakeTimers();
    rs.setSystemTime(new Date('2026-08-09T12:00:00.000Z'));
  });

  afterEach(() => {
    rs.useRealTimers();
  });

  test('returns null secondsLeft when expires_at is missing', () => {
    membership = { status: 'QUEUED' };

    const { result } = renderHook(() => useQueueStatus('product-1'));

    expect(result.current.membership).toEqual({ status: 'QUEUED' });
    expect(result.current.secondsLeft).toBeNull();
  });

  test('counts down secondsLeft from expires_at', () => {
    membership = {
      status: 'RIGHT_ACTIVE',
      expires_at: '2026-08-09T12:00:05.000Z',
    };

    const { result } = renderHook(() => useQueueStatus('product-1'));

    expect(result.current.secondsLeft).toBe(5);

    act(() => {
      rs.advanceTimersByTime(1000);
    });

    expect(result.current.secondsLeft).toBe(4);
  });
});

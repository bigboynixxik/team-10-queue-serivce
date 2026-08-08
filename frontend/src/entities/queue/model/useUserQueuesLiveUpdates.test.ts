import { afterEach, beforeEach, describe, expect, rs, test } from '@rstest/core';
import { act } from '@testing-library/react';

import { createTestQueryClient, renderHookWithProviders } from '@test/render';

type Listener = (event: MessageEvent<string>) => void;

class FakeEventSource {
  static instances: FakeEventSource[] = [];
  readonly url: string;
  onerror: (() => void) | null = null;
  private readonly listeners = new Map<string, Listener[]>();
  close = rs.fn();

  constructor(url: string) {
    this.url = url;
    FakeEventSource.instances.push(this);
  }

  addEventListener(type: string, listener: Listener) {
    const list = this.listeners.get(type) ?? [];
    list.push(listener);
    this.listeners.set(type, list);
  }

  emit(type: string, data: string) {
    for (const listener of this.listeners.get(type) ?? []) {
      listener(new MessageEvent(type, { data }));
    }
  }
}

rs.mock('@shared/config', () => ({
  API_BASE_URL: 'https://api.example.com/api/v1',
}));

const { useUserQueuesLiveUpdates } = await import('./useUserQueuesLiveUpdates');
const { queueMembershipQueryKey, userQueuesQueryKey } = await import('./queries');

describe('useUserQueuesLiveUpdates', () => {
  beforeEach(() => {
    FakeEventSource.instances = [];
    rs.stubGlobal('EventSource', FakeEventSource);
    rs.useFakeTimers();
  });

  afterEach(() => {
    rs.unstubAllGlobals();
    rs.useRealTimers();
  });

  test('does not open EventSource without userId', () => {
    renderHookWithProviders(() => useUserQueuesLiveUpdates(''));

    expect(FakeEventSource.instances).toHaveLength(0);
  });

  test('builds sse url and syncs memberships on update', () => {
    const queryClient = createTestQueryClient();
    const onUpdate = rs.fn();

    renderHookWithProviders(() => useUserQueuesLiveUpdates('user-1', { onUpdate }), {
      queryClient,
    });

    const source = FakeEventSource.instances[0];
    expect(source.url).toBe('https://api.example.com/api/v1/me/queues?user_id=user-1');

    const queues = [
      {
        product_id: 'p1',
        status: 'QUEUED',
        position: 2,
        token: 'tok',
      },
    ];

    act(() => {
      source.emit('update', JSON.stringify(queues));
    });

    expect(queryClient.getQueryData(userQueuesQueryKey('user-1'))).toEqual(queues);
    expect(queryClient.getQueryData(queueMembershipQueryKey('p1'))).toEqual({
      status: 'QUEUED',
      token: 'tok',
      quantity: undefined,
      available_quantity: undefined,
      expires_at: undefined,
    });
    expect(onUpdate).toHaveBeenCalledWith(queues);
  });

  test('invalidates on parse error and reconnects on error', async () => {
    const queryClient = createTestQueryClient();
    const invalidateSpy = rs.spyOn(queryClient, 'invalidateQueries');

    renderHookWithProviders(() => useUserQueuesLiveUpdates('user-1'), { queryClient });

    const source = FakeEventSource.instances[0];

    act(() => {
      source.emit('update', '{bad-json');
    });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: userQueuesQueryKey('user-1') });

    act(() => {
      source.onerror?.();
    });

    await act(async () => {
      rs.advanceTimersByTime(1000);
    });

    expect(FakeEventSource.instances).toHaveLength(2);
  });

  test('closes source on unmount', () => {
    const { unmount } = renderHookWithProviders(() => useUserQueuesLiveUpdates('user-1'));
    const source = FakeEventSource.instances[0];

    unmount();

    expect(source.close).toHaveBeenCalled();
  });
});

import { beforeEach, describe, expect, rs, test } from '@rstest/core';

import { createAxiosMock } from '@test/mockAxios';

const axiosMock = createAxiosMock();

rs.mock('@shared/config', () => ({
  API_BASE_URL: 'https://api.example.com/api/v1',
  USER_ID_STORAGE_KEY: 'queue-service:user-id',
}));

rs.mock('axios', () => ({
  default: {
    create: axiosMock.create,
  },
}));

const { queueApi } = await import('./QueueApi');

const membership = {
  status: 'RIGHT_ACTIVE',
  token: 'tok',
  quantity: 2,
  expires_at: '2026-08-09T10:00:00.000Z',
};

describe('QueueApi', () => {
  beforeEach(() => {
    axiosMock.get.mockReset();
    axiosMock.post.mockReset();
    axiosMock.patch.mockReset();
    axiosMock.del.mockReset();
  });

  test('join posts payload and parses membership', async () => {
    axiosMock.post.mockResolvedValue({ data: membership });

    await expect(queueApi.join('p1', { quantity: 2 })).resolves.toMatchObject({
      status: 'RIGHT_ACTIVE',
      token: 'tok',
    });
    expect(axiosMock.post).toHaveBeenCalledWith('/p1/members', { quantity: 2 }, undefined);
  });

  test('join rejects invalid payload before request', async () => {
    await expect(queueApi.join('p1', { quantity: 0 })).rejects.toThrow();
    expect(axiosMock.post).not.toHaveBeenCalled();
  });

  test('getMe and getStats parse responses', async () => {
    axiosMock.get.mockResolvedValueOnce({ data: { status: 'QUEUED' } }).mockResolvedValueOnce({
      data: {
        waiting: 0,
        holding_right: 1,
        pending_offer: 0,
        available: 2,
        product_count: 5,
      },
    });

    await expect(queueApi.getMe('p1')).resolves.toEqual({ status: 'QUEUED' });
    await expect(queueApi.getStats('p1')).resolves.toMatchObject({ product_count: 5 });
  });

  test('acceptOffer patches and declineOffer deletes', async () => {
    axiosMock.patch.mockResolvedValue({ data: membership });
    axiosMock.del.mockResolvedValue({ data: undefined });

    await expect(queueApi.acceptOffer('p1', { quantity: 1 })).resolves.toMatchObject({
      status: 'RIGHT_ACTIVE',
    });
    await expect(queueApi.declineOffer('p1')).resolves.toBeUndefined();

    expect(axiosMock.patch).toHaveBeenCalledWith('/p1/members/me', { quantity: 1 }, undefined);
    expect(axiosMock.del).toHaveBeenCalledWith('/p1/members/me', undefined);
  });
});

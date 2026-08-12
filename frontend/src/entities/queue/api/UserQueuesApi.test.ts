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

const { userQueuesApi } = await import('./UserQueuesApi');

describe('UserQueuesApi', () => {
  beforeEach(() => {
    axiosMock.get.mockReset();
  });

  test('getAll parses user queues list', async () => {
    axiosMock.get.mockResolvedValue({
      data: [{ product_id: 'p1', status: 'QUEUED', position: 1 }],
    });

    await expect(userQueuesApi.getAll()).resolves.toEqual([
      { product_id: 'p1', status: 'QUEUED', position: 1 },
    ]);
    // baseURL уже /me/queues — не дублируем путь
    expect(axiosMock.get).toHaveBeenCalledWith('', undefined);
  });

  test('getAll accepts empty list and rejects invalid items', async () => {
    axiosMock.get.mockResolvedValueOnce({ data: [] });
    await expect(userQueuesApi.getAll()).resolves.toEqual([]);

    axiosMock.get.mockResolvedValueOnce({ data: [{ status: 'QUEUED' }] });
    await expect(userQueuesApi.getAll()).rejects.toThrow();
  });
});

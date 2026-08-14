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

const { sellerStatsApi } = await import('./SellerStatsApi');
const { NotFoundError } = await import('@shared/api');

const metrics = {
  total_stock: 10,
  total_contenders: 200,
  used_rights_count: 8,
  expired_rights_count: 2,
  soldout_count: 145,
  dropoff_count: 45,
  avg_payment_time: 75,
  avg_dropoff_time: 120,
};

describe('SellerStatsApi', () => {
  beforeEach(() => {
    axiosMock.get.mockReset();
  });

  test('byProductId fetches and parses metrics', async () => {
    axiosMock.get.mockResolvedValue({ data: metrics });

    await expect(sellerStatsApi.byProductId('wireless-headphones')).resolves.toEqual(metrics);
    expect(axiosMock.get).toHaveBeenCalledWith('/wireless-headphones/metrics', undefined);
  });

  test('byProductId accepts null average times', async () => {
    axiosMock.get.mockResolvedValue({
      data: { ...metrics, avg_payment_time: null, avg_dropoff_time: null },
    });

    await expect(sellerStatsApi.byProductId('p1')).resolves.toMatchObject({
      avg_payment_time: null,
      avg_dropoff_time: null,
    });
  });

  test('byProductId rejects invalid response', async () => {
    axiosMock.get.mockResolvedValue({ data: { total_stock: 'bad' } });

    await expect(sellerStatsApi.byProductId('p1')).rejects.toThrow();
  });

  test('byProductId propagates NotFoundError', async () => {
    axiosMock.get.mockRejectedValue(new NotFoundError('Статистика не найдена'));

    await expect(sellerStatsApi.byProductId('missing')).rejects.toMatchObject({
      code: 404,
      message: 'Статистика не найдена',
    });
  });
});

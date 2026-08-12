import { describe, expect, test } from '@rstest/core';

import { sellerStatsApi } from './SellerStatsApi';
import { SellerStatsSchema } from './type';

describe('SellerStatsApi', () => {
  test('returns stats by product id', async () => {
    const stats = await sellerStatsApi.byProductId('wireless-headphones');

    expect(SellerStatsSchema.parse(stats).product_id).toBe('wireless-headphones');
    expect(stats.lost_revenue).toBe(stats.sold_out_count * stats.price);
  });

  test('throws NotFoundError for missing product', async () => {
    await expect(sellerStatsApi.byProductId('missing-product')).rejects.toMatchObject({
      code: 404,
      message: 'Статистика не найдена',
    });
  });
});

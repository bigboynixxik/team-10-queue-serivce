import { NotFoundError } from '@shared/api';

import statsJson from './mocks/stats.json';
import { type SellerStats, SellerStatsListSchema } from './type';

class SellerStatsApi {
  private readonly stats = SellerStatsListSchema.parse(statsJson);

  public async byProductId(productId: string): Promise<SellerStats> {
    const stats = this.stats.find((item) => item.product_id === productId);

    if (!stats) throw new NotFoundError('Статистика не найдена');

    return stats;
  }
}

export const sellerStatsApi = new SellerStatsApi();

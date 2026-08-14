import { HttpClient } from '@shared/api';

import { type SellerStats, SellerStatsSchema } from './type';

class SellerStatsApi extends HttpClient {
  constructor() {
    super('seller/products');
  }

  public async byProductId(productId: string): Promise<SellerStats> {
    return SellerStatsSchema.parse(await this.get<unknown>({ uri: `/${productId}/metrics` }));
  }
}

export const sellerStatsApi = new SellerStatsApi();

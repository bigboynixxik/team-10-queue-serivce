import { NotFoundError } from '@shared/api';

import productsJson from './mocks/products.json';
import { type Product, ProductsSchema } from './type';

class ProductApi {
  private readonly products = ProductsSchema.parse(productsJson);

  public async list(): Promise<Product[]> {
    return this.products;
  }

  public async listMine(): Promise<Product[]> {
    return this.products;
  }

  public async byId(id: string): Promise<Product> {
    const product = this.products.find((candidate) => candidate.id === id);

    if (!product) throw new NotFoundError('Товар не найден');

    return product;
  }
}

export const productApi = new ProductApi();

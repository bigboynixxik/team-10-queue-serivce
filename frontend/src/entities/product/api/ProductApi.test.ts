import { describe, expect, test } from '@rstest/core';

import { productApi } from './ProductApi';

describe('ProductApi', () => {
  test('lists products from fixture', async () => {
    const products = await productApi.list();

    expect(products.length).toBeGreaterThan(0);
    expect(products[0]).toMatchObject({
      id: expect.any(String),
      title: expect.any(String),
      price: expect.any(Number),
      image: expect.any(String),
    });
  });

  test('returns product by id', async () => {
    const product = await productApi.byId('wireless-headphones');

    expect(product.id).toBe('wireless-headphones');
    expect(product.title).toContain('Sennheiser');
  });

  test('throws NotFoundError for missing product', async () => {
    await expect(productApi.byId('missing-product')).rejects.toMatchObject({
      code: 404,
      message: 'Товар не найден',
    });
  });
});

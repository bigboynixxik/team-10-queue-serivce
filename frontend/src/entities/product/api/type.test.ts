import { describe, expect, test } from '@rstest/core';

import productsJson from './mocks/products.json';
import { ProductSchema, ProductsSchema } from './type';

const validProduct = {
  id: 'p1',
  title: 'Title',
  description: 'Description',
  price: 100,
  image: 'https://example.com/image.png',
};

describe('product api schemas', () => {
  test('accepts valid product', () => {
    expect(ProductSchema.parse(validProduct)).toEqual(validProduct);
  });

  test('rejects empty strings and invalid price/image', () => {
    expect(() => ProductSchema.parse({ ...validProduct, id: '' })).toThrow();
    expect(() => ProductSchema.parse({ ...validProduct, price: 0 })).toThrow();
    expect(() => ProductSchema.parse({ ...validProduct, image: 'not-a-url' })).toThrow();
  });

  test('accepts root-relative asset paths', () => {
    expect(
      ProductSchema.parse({ ...validProduct, image: '/assets/wireless-headphones.jpg' }).image,
    ).toBe('/assets/wireless-headphones.jpg');
  });

  test('parses products fixture', () => {
    const products = ProductsSchema.parse(productsJson);

    expect(products.length).toBeGreaterThan(0);
    expect(products[0]).toMatchObject({ id: expect.any(String), price: expect.any(Number) });
  });
});

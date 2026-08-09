import { z } from 'zod';

export const ProductSchema = z.object({
  id: z.string().min(1),
  title: z.string().min(1),
  description: z.string().min(1),
  price: z.number().int().positive(),
  image: z
    .string()
    .min(1)
    .refine(
      (value) => value.startsWith('/') || z.string().url().safeParse(value).success,
      { message: 'Invalid image url' },
    ),
});

export const ProductsSchema = z.array(ProductSchema);

export type Product = z.infer<typeof ProductSchema>;

import { productQueries } from '@entities/product';
import { useQuery } from '@tanstack/react-query';

export const useProductList = () => useQuery(productQueries.list());

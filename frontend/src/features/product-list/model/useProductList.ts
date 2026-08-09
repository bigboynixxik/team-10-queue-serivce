import { useQuery } from '@tanstack/react-query';

import { productQueries } from '@entities/product';

export const useProductList = () => useQuery(productQueries.list());

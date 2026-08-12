import { useNavigate } from 'react-router-dom';

import type { Product } from '@entities/product';
import { appPath } from '@shared/config';

export const useSellerProductCard = (product: Product) => {
  const navigate = useNavigate();

  return {
    openProduct: () => navigate(appPath(`/order-info/${product.id}`)),
  };
};

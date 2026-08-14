import { useQuery } from '@tanstack/react-query';
import { useParams } from 'react-router-dom';

import { productQueries } from '@entities/product';
import { useUserStore } from '@entities/user';

export const useOrderInfoPage = () => {
  const { productId = '' } = useParams();
  const role = useUserStore.use.role();
  const {
    data: product,
    isPending,
    isError,
  } = useQuery({ ...productQueries.byId(productId), enabled: Boolean(productId) });

  return {
    product,
    role,
    isSeller: role === 'seller',
    isPending: Boolean(productId) && isPending,
    isError: !productId || isError,
  };
};

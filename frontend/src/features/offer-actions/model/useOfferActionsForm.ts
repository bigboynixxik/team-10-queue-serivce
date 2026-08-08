import { useState } from 'react';

import { useOfferActions } from './useOfferActions';

export const useOfferActionsForm = (productId: string, availableQuantity: number) => {
  const [quantity, setQuantity] = useState(availableQuantity);
  const { accept, decline, isPending } = useOfferActions(productId);

  const setValidQuantity = (value: number | null) => {
    const nextQuantity = value ?? 1;
    setQuantity(Math.min(Math.max(nextQuantity, 1), availableQuantity));
  };

  return {
    quantity,
    setQuantity: setValidQuantity,
    accept: () => accept({ quantity }),
    decline,
    isPending,
  };
};

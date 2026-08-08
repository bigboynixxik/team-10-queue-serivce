import { queryClient } from '@shared/lib';
import { mutationOptions } from '@tanstack/react-query';

import { queueApi } from '../api/QueueApi';
import type { AcceptOfferPayload, JoinPayload } from '../api/type';
import { queueMembershipQueryKey } from './queries';

const invalidateMembership = (productId: string) =>
  queryClient.invalidateQueries({ queryKey: queueMembershipQueryKey(productId) });

export const queueMutations = {
  join: (productId: string) =>
    mutationOptions({
      mutationFn: (payload: JoinPayload) => queueApi.join(productId, payload),
      onSuccess: () => invalidateMembership(productId),
    }),
  acceptOffer: (productId: string) =>
    mutationOptions({
      mutationFn: (payload: AcceptOfferPayload) => queueApi.acceptOffer(productId, payload),
      onSuccess: () => invalidateMembership(productId),
    }),
  declineOffer: (productId: string) =>
    mutationOptions({
      mutationFn: () => queueApi.declineOffer(productId),
      onSuccess: () => invalidateMembership(productId),
    }),
};

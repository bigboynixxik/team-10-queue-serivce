import { mutationOptions } from '@tanstack/react-query';

import { rightsApi } from '../api/RightsApi';

export const rightsMutations = {
  validate: () =>
    mutationOptions({
      mutationFn: (token: string) => rightsApi.validate(token),
    }),
};

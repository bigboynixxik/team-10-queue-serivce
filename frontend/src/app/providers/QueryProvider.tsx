import { QueryClientProvider, type QueryClientProviderProps } from '@tanstack/react-query';

import { queryClient } from '@shared/lib';

type QueryProviderProps = Pick<QueryClientProviderProps, 'children'>;

export const QueryProvider = ({ children }: QueryProviderProps): React.JSX.Element => {
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
};

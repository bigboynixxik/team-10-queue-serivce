import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import {
  type RenderHookOptions,
  type RenderOptions,
  render,
  renderHook,
} from '@testing-library/react';
import type { ReactElement, ReactNode } from 'react';

const createTestQueryClient = () =>
  new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0 },
      mutations: { retry: false },
    },
  });

type ProvidersProps = {
  children: ReactNode;
  queryClient?: QueryClient;
};

const Providers = ({ children, queryClient = createTestQueryClient() }: ProvidersProps) => (
  <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
);

export const renderWithProviders = (
  ui: ReactElement,
  options?: Omit<RenderOptions, 'wrapper'> & { queryClient?: QueryClient },
) => {
  const { queryClient, ...renderOptions } = options ?? {};

  return render(ui, {
    wrapper: ({ children }) => <Providers queryClient={queryClient}>{children}</Providers>,
    ...renderOptions,
  });
};

export const renderHookWithProviders = <TResult, TProps>(
  hook: (props: TProps) => TResult,
  options?: Omit<RenderHookOptions<TProps>, 'wrapper'> & { queryClient?: QueryClient },
) => {
  const { queryClient, ...hookOptions } = options ?? {};

  return renderHook(hook, {
    wrapper: ({ children }) => <Providers queryClient={queryClient}>{children}</Providers>,
    ...hookOptions,
  });
};

export { createTestQueryClient };

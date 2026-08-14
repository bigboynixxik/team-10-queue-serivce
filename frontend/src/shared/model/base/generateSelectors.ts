import type { StoreApi, UseBoundStore } from 'zustand';
import { useShallow } from 'zustand/shallow';

// https://zustand.docs.pmnd.rs/guides/auto-generating-selectors

type Selectors<T> = { [K in keyof T]: () => T[K] };

type WithSelectors<S extends UseBoundStore<StoreApi<object>>> = S & {
  use: Selectors<ReturnType<S['getState']>>;
};

export const createSelectors = <S extends UseBoundStore<StoreApi<object>>>(
  store: S,
): WithSelectors<S> => {
  const use: Record<string, () => unknown> = {};

  for (const key of Object.keys(store.getState())) {
    use[key] = () => store(useShallow((state) => state[key as keyof typeof state]));
  }

  // форму use собираем в рантайме поэтому каст нужен
  return Object.assign(store, { use }) as unknown as WithSelectors<S>;
};

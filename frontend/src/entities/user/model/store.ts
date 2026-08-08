import { USER_ID_STORAGE_KEY } from '@shared/config';
import { type BaseStoreActions, create, createSelectors } from '@shared/model';

import { createUserId } from './utils';

type State = {
  userId: string;
};

type Actions = BaseStoreActions & {
  setUserId: (userId: string) => void;
};

type Store = State & Actions;

const initialUserId = createUserId();

const initialState: State = {
  userId: initialUserId,
};

const useUserStoreBase = create<Store>()((set) => ({
  ...initialState,

  setUserId: (userId) => {
    localStorage.setItem(USER_ID_STORAGE_KEY, userId);
    set({ userId });
  },

  reset: () => set({ userId: initialUserId }),
}));

export const useUserStore = createSelectors(useUserStoreBase);

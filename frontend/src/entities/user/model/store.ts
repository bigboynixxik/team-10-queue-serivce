import { USER_ID_STORAGE_KEY, USER_ROLE_STORAGE_KEY } from '@shared/config';
import { type BaseStoreActions, create, createSelectors } from '@shared/model';

import { readStoredRole, type UserRole } from './role';
import { createUserId } from './utils';

type State = {
  userId: string;
  role: UserRole;
};

type Actions = BaseStoreActions & {
  setUserId: (userId: string) => void;
  setRole: (role: UserRole) => void;
};

type Store = State & Actions;

const initialUserId = createUserId();
const initialRole = readStoredRole();

const initialState: State = {
  userId: initialUserId,
  role: initialRole,
};

const useUserStoreBase = create<Store>()((set) => ({
  ...initialState,

  setUserId: (userId) => {
    localStorage.setItem(USER_ID_STORAGE_KEY, userId);
    set({ userId });
  },

  setRole: (role) => {
    localStorage.setItem(USER_ROLE_STORAGE_KEY, role);
    set({ role });
  },

  reset: () => {
    localStorage.setItem(USER_ROLE_STORAGE_KEY, initialRole);
    set({ userId: initialUserId, role: initialRole });
  },
}));

export const useUserStore = createSelectors(useUserStoreBase);

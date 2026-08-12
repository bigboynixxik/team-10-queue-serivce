import { useNavigate } from 'react-router-dom';

import { isUserRole, type UserRole, useUserStore } from '@entities/user';
import { appPath } from '@shared/config';

export const useRoleSelector = () => {
  const navigate = useNavigate();
  const role = useUserStore.use.role();
  const setRole = useUserStore.use.setRole();

  const onChange = (value: string) => {
    if (!isUserRole(value)) return;

    setRole(value);

    if (value === 'buyer') {
      navigate(appPath());
    }
  };

  return {
    role,
    onChange,
    options: [
      { value: 'buyer' as UserRole, label: 'Пользователь' },
      { value: 'seller' as UserRole, label: 'Продавец' },
    ],
  };
};

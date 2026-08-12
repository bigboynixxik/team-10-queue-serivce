import { cn } from '@shared/lib';

import { useRoleSelector } from '../model/useRoleSelector';
import styles from './RoleSelector.module.css';

const bem = cn('RoleSelector');

export const RoleSelector = (): React.JSX.Element => {
  const { role, onChange, options } = useRoleSelector();

  return (
    <label className={styles[bem()]}>
      <span className={styles[bem('label')]}>Роль</span>
      <select
        aria-label="Выбор роли"
        className={styles[bem('control')]}
        onChange={(event) => onChange(event.target.value)}
        value={role}
      >
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
    </label>
  );
};

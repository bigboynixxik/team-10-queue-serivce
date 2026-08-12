import { beforeEach, describe, expect, rs, test } from '@rstest/core';
import { act, renderHook } from '@testing-library/react';

const setRole = rs.fn();
const navigate = rs.fn();
let role = 'buyer';

rs.mock('react-router-dom', () => ({
  useNavigate: () => navigate,
}));

rs.mock('@entities/user', () => ({
  isUserRole: (value: string) => value === 'buyer' || value === 'seller',
  useUserStore: {
    use: {
      role: () => role,
      setRole: () => setRole,
    },
  },
}));

const { useRoleSelector } = await import('./useRoleSelector');

describe('useRoleSelector', () => {
  beforeEach(() => {
    role = 'seller';
    setRole.mockReset();
    navigate.mockReset();
  });

  test('exposes role options and changes role', () => {
    const { result } = renderHook(() => useRoleSelector());

    expect(result.current.role).toBe('seller');
    expect(result.current.options).toHaveLength(2);

    act(() => {
      result.current.onChange('seller');
    });

    expect(setRole).toHaveBeenCalledWith('seller');
    expect(navigate).not.toHaveBeenCalled();
  });

  test('navigates home when switching to buyer', () => {
    const { result } = renderHook(() => useRoleSelector());

    act(() => {
      result.current.onChange('buyer');
    });

    expect(setRole).toHaveBeenCalledWith('buyer');
    expect(navigate).toHaveBeenCalledWith('/avito');
  });

  test('ignores invalid role values', () => {
    const { result } = renderHook(() => useRoleSelector());

    act(() => {
      result.current.onChange('admin');
    });

    expect(setRole).not.toHaveBeenCalled();
    expect(navigate).not.toHaveBeenCalled();
  });
});

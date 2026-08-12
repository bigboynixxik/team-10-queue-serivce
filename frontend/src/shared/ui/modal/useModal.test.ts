import { describe, expect, test } from '@rstest/core';
import { act, renderHook } from '@testing-library/react';

import { useModal } from './useModal';

describe('useModal', () => {
  test('opens and closes', () => {
    const { result } = renderHook(() => useModal());

    expect(result.current.isOpen).toBe(false);

    act(() => {
      result.current.open();
    });
    expect(result.current.isOpen).toBe(true);

    act(() => {
      result.current.close();
    });
    expect(result.current.isOpen).toBe(false);
  });
});

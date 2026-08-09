import { useCallback, useEffect, useMemo, useRef, useState } from 'react';

import type { Toast, ToastContent, ToastContextValue, ToastOptions } from './ToastContext';
import { ERROR_TOAST_DURATION, STATUS_TOAST_DURATION, toastKey } from './toastKey';

export const useToastState = (): {
  toasts: Toast[];
  dismiss: (id: number) => void;
  value: ToastContextValue;
} => {
  const [toasts, setToasts] = useState<Toast[]>([]);
  const timeoutIds = useRef(new Map<number, number>());
  const visibleKeys = useRef(new Set<string>());

  const dismiss = useCallback((id: number) => {
    const timeoutId = timeoutIds.current.get(id);

    if (timeoutId) window.clearTimeout(timeoutId);

    timeoutIds.current.delete(id);
    setToasts((current) => {
      const toast = current.find((item) => item.id === id);

      if (toast) visibleKeys.current.delete(toast.key);

      return current.filter((item) => item.id !== id);
    });
  }, []);

  const addToast = useCallback(
    (content: ToastContent, variant: Toast['variant'], duration: number) => {
      const key = toastKey(content, variant);

      // одинаковый тост не копим пока висит предыдущий
      if (visibleKeys.current.has(key)) return;

      visibleKeys.current.add(key);

      const id = Date.now() + Math.random();

      setToasts((current) => [...current, { id, key, ...content, variant }]);
      timeoutIds.current.set(
        id,
        window.setTimeout(() => dismiss(id), duration),
      );
    },
    [dismiss],
  );

  useEffect(
    () => () => {
      for (const timeoutId of timeoutIds.current.values()) {
        window.clearTimeout(timeoutId);
      }
    },
    [],
  );

  const value = useMemo(
    () => ({
      error: (message: string) =>
        addToast({ title: 'Ошибка', description: message }, 'error', ERROR_TOAST_DURATION),
      info: (content: ToastContent, options?: ToastOptions) =>
        addToast(content, 'info', options?.duration ?? STATUS_TOAST_DURATION),
      success: (content: ToastContent, options?: ToastOptions) =>
        addToast(content, 'success', options?.duration ?? STATUS_TOAST_DURATION),
    }),
    [addToast],
  );

  return { toasts, dismiss, value };
};

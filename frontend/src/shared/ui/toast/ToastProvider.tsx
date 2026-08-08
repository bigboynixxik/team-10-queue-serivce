import { cn } from '@shared/lib';
import type { PropsWithChildren } from 'react';
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';
import { createPortal } from 'react-dom';

import { Alert } from '../alert/Alert';
import styles from './ToastProvider.module.css';

const bem = cn('ToastProvider');
const ERROR_TOAST_DURATION = 5000;
const STATUS_TOAST_DURATION = 25000;

type Toast = {
  id: number;
  key: string;
  title: string;
  description: string;
  variant: 'error' | 'info' | 'success';
};

type ToastContent = {
  title: string;
  description: string;
};

type ToastOptions = {
  duration?: number;
};

type ToastContextValue = {
  error: (message: string) => void;
  info: (content: ToastContent, options?: ToastOptions) => void;
  success: (content: ToastContent, options?: ToastOptions) => void;
};

const ToastContext = createContext<ToastContextValue | null>(null);

const toastKey = (content: ToastContent, variant: Toast['variant']): string =>
  `${variant}:${content.title}:${content.description}`;

export const ToastProvider = ({ children }: PropsWithChildren): React.JSX.Element => {
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

  return (
    <ToastContext.Provider value={value}>
      {children}
      {createPortal(
        <div aria-live="polite" className={styles[bem()]}>
          {toasts.map((toast) => (
            <div className={styles[bem('item')]} key={toast.id}>
              <Alert
                description={toast.description}
                onClose={() => dismiss(toast.id)}
                title={toast.title}
                variant={toast.variant}
              />
            </div>
          ))}
        </div>,
        document.body,
      )}
    </ToastContext.Provider>
  );
};

export const useToast = (): ToastContextValue => {
  const context = useContext(ToastContext);

  if (!context) throw new Error('useToast must be used within ToastProvider');

  return context;
};

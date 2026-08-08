import { cn } from '@shared/lib';
import type { PropsWithChildren } from 'react';
import { createContext, useContext, useMemo, useState } from 'react';
import { createPortal } from 'react-dom';

import styles from './ToastProvider.module.css';

const bem = cn('ToastProvider');
const TOAST_DURATION = 5000;

type Toast = {
  id: number;
  message: string;
};

type ToastContextValue = {
  error: (message: string) => void;
};

const ToastContext = createContext<ToastContextValue | null>(null);

export const ToastProvider = ({ children }: PropsWithChildren): React.JSX.Element => {
  const [toasts, setToasts] = useState<Toast[]>([]);

  const value = useMemo(
    () => ({
      error: (message: string) => {
        const id = Date.now() + Math.random();
        setToasts((current) => [...current, { id, message }]);
        window.setTimeout(() => {
          setToasts((current) => current.filter((toast) => toast.id !== id));
        }, TOAST_DURATION);
      },
    }),
    [],
  );

  return (
    <ToastContext.Provider value={value}>
      {children}
      {createPortal(
        <div aria-live="polite" className={styles[bem()]}>
          {toasts.map((toast) => (
            <div className={styles[bem('item')]} key={toast.id} role="alert">
              {toast.message}
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

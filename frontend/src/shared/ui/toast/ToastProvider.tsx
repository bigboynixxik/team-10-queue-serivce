import type { PropsWithChildren } from 'react';
import { createPortal } from 'react-dom';

import { cn } from '@shared/lib';

import { Alert } from '../alert/Alert';
import { ToastContext } from './ToastContext';
import styles from './ToastProvider.module.css';
import { useToastState } from './useToastState';

const bem = cn('ToastProvider');

export const ToastProvider = ({ children }: PropsWithChildren): React.JSX.Element => {
  const { toasts, dismiss, value } = useToastState();

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

export { useToast } from './useToast';

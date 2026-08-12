import { type PropsWithChildren, type ReactNode, useEffect } from 'react';
import { createPortal } from 'react-dom';

import { cn } from '@shared/lib';

import styles from './Modal.module.css';

const bem = cn('Modal');

type Props = PropsWithChildren<{
  open: boolean;
  title: string;
  description?: ReactNode;
  onClose: () => void;
}>;

export const Modal = ({
  open,
  title,
  description,
  onClose,
  children,
}: Props): React.JSX.Element | null => {
  useEffect(() => {
    if (!open) return;

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose();
    };

    document.addEventListener('keydown', onKeyDown);
    return () => document.removeEventListener('keydown', onKeyDown);
  }, [open, onClose]);

  if (!open || typeof document === 'undefined') return null;

  return createPortal(
    <div aria-modal className={styles[bem()]} role="dialog">
      <button
        aria-label="Закрыть"
        className={styles[bem('backdrop')]}
        onClick={onClose}
        type="button"
      />
      <div className={styles[bem('dialog')]}>
        <h2 className={styles[bem('title')]}>{title}</h2>
        {description && <div className={styles[bem('description')]}>{description}</div>}
        <div className={styles[bem('actions')]}>{children}</div>
      </div>
    </div>,
    document.body,
  );
};

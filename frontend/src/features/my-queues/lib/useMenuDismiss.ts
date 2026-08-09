import { type RefObject, useEffect } from 'react';

type UseMenuDismissParams = {
  menuRef: RefObject<HTMLElement | null>;
  isOpen: boolean;
  onDismiss: () => void;
};

export const useMenuDismiss = ({ menuRef, isOpen, onDismiss }: UseMenuDismissParams): void => {
  useEffect(() => {
    if (!isOpen) return;

    const closeOnOutsideClick = (event: MouseEvent) => {
      if (!menuRef.current?.contains(event.target as Node)) {
        onDismiss();
      }
    };

    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        onDismiss();
      }
    };

    document.addEventListener('mousedown', closeOnOutsideClick);
    document.addEventListener('keydown', closeOnEscape);

    return () => {
      document.removeEventListener('mousedown', closeOnOutsideClick);
      document.removeEventListener('keydown', closeOnEscape);
    };
  }, [isOpen, menuRef, onDismiss]);
};

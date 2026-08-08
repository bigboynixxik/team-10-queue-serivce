import { getApiErrorMessages } from '@shared/api';
import { useToast } from '@ui';

export type ErrorNotifier = (error: unknown, fallback?: string) => void;

/**
 * Single funnel for turning API rejections into user-facing toasts, so every
 * feature reports failures the same way.
 */
export const useErrorNotifier = (): ErrorNotifier => {
  const toast = useToast();

  return (error, fallback) => {
    for (const text of getApiErrorMessages(error, fallback)) {
      toast.error(text);
    }
  };
};

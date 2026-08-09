import { getApiErrorMessages } from '@shared/api';
import { useToast } from '@ui';

export type ErrorNotifier = (error: unknown, fallback?: string) => void;

/** единая точка показа ошибок api через toast */
export const useErrorNotifier = (): ErrorNotifier => {
  const toast = useToast();

  return (error, fallback) => {
    for (const text of getApiErrorMessages(error, fallback)) {
      toast.error(text);
    }
  };
};

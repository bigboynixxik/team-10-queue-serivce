import { createContext } from 'react';

export type Toast = {
  id: number;
  key: string;
  title: string;
  description: string;
  variant: 'error' | 'info' | 'success';
};

export type ToastContent = {
  title: string;
  description: string;
};

export type ToastOptions = {
  duration?: number;
};

export type ToastContextValue = {
  error: (message: string) => void;
  info: (content: ToastContent, options?: ToastOptions) => void;
  success: (content: ToastContent, options?: ToastOptions) => void;
};

export const ToastContext = createContext<ToastContextValue | null>(null);

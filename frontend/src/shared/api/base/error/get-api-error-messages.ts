import { z } from 'zod';

import HttpError from './HttpError';

const DEFAULT_FALLBACK_MESSAGE = 'Не удалось выполнить запрос. Попробуйте еще раз.';

const errorMessageSchema = z
  .union([z.string(), z.array(z.string())])
  .transform((value) =>
    (Array.isArray(value) ? value : [value]).map((message) => message.trim()).filter(Boolean),
  );

const errorResponseSchema = z.object({
  message: errorMessageSchema,
});

const normalizeMessages = (value: unknown): string[] => {
  const parsed = errorMessageSchema.safeParse(value);
  return parsed.success ? parsed.data : [];
};

const getMessagesFromResponseData = (data: unknown): string[] => {
  const parsed = errorResponseSchema.safeParse(data);
  return parsed.success ? parsed.data.message : [];
};

export const getApiErrorMessages = (
  error: unknown,
  fallback = DEFAULT_FALLBACK_MESSAGE,
): string[] => {
  const fallbackMessage = fallback.trim() || DEFAULT_FALLBACK_MESSAGE;

  if (error instanceof HttpError && error.messages.length > 0) {
    return error.messages;
  }

  if (typeof error === 'object' && error !== null) {
    const responseData = (error as { response?: { data?: unknown } }).response?.data;
    const responseMessages = getMessagesFromResponseData(responseData);

    if (responseMessages.length > 0) return responseMessages;

    const nestedMessages = normalizeMessages((error as { message?: unknown }).message);

    if (nestedMessages.length > 0) return nestedMessages;
  }

  if (error instanceof Error) {
    const errorMessages = normalizeMessages(error.message);

    if (errorMessages.length > 0) return errorMessages;
  }

  return [fallbackMessage];
};

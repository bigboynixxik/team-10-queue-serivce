import { describe, expect, test } from '@rstest/core';

import { getApiErrorMessages } from './get-api-error-messages';
import HttpError from './HttpError';

describe('getApiErrorMessages', () => {
  test('returns HttpError messages', () => {
    expect(getApiErrorMessages(new HttpError(400, ['a', 'b']))).toEqual(['a', 'b']);
  });

  test('reads response.data.message string', () => {
    expect(getApiErrorMessages({ response: { data: { message: '  fail  ' } } })).toEqual(['fail']);
  });

  test('reads response.data.message array', () => {
    expect(
      getApiErrorMessages({ response: { data: { message: [' first ', '', 'second'] } } }),
    ).toEqual(['first', 'second']);
  });

  test('reads nested Error.message', () => {
    expect(getApiErrorMessages({ message: 'nested' })).toEqual(['nested']);
  });

  test('reads Error instance message', () => {
    expect(getApiErrorMessages(new Error('boom'))).toEqual(['boom']);
  });

  test('uses fallback for unknown values', () => {
    expect(getApiErrorMessages(null)).toEqual(['Не удалось выполнить запрос. Попробуйте еще раз.']);
  });

  test('uses custom fallback and ignores blank fallback', () => {
    expect(getApiErrorMessages(undefined, 'Кастомная ошибка')).toEqual(['Кастомная ошибка']);
    expect(getApiErrorMessages(undefined, '   ')).toEqual([
      'Не удалось выполнить запрос. Попробуйте еще раз.',
    ]);
  });
});

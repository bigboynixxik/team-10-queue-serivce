import { describe, expect, test } from '@rstest/core';

import HttpError from './HttpError';

describe('HttpError', () => {
  test('normalizes string message', () => {
    const error = new HttpError(400, '  bad request  ');

    expect(error.messages).toEqual(['bad request']);
    expect(error.message).toBe('bad request');
    expect(error.code).toBe(400);
    expect(error.name).toBe('HttpError');
  });

  test('normalizes message array and trims empties', () => {
    const error = new HttpError(500, ['  one ', '', ' two']);

    expect(error.messages).toEqual(['one', 'two']);
    expect(error.message).toBe('one');
  });

  test('falls back when messages are empty', () => {
    const error = new HttpError(500, ['  ', '']);

    expect(error.messages).toEqual(['Произошла ошибка']);
  });
});

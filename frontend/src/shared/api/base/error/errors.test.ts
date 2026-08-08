import { describe, expect, test } from '@rstest/core';

import BadRequest from './BadRequest';
import HttpError from './HttpError';
import NoAccess from './NoAccess';
import NotFoundError from './NotFoundError';
import ServerError from './ServerError';

describe('api error classes', () => {
  test('BadRequest uses status 400', () => {
    const error = new BadRequest(['bad']);

    expect(error).toBeInstanceOf(HttpError);
    expect(error.code).toBe(400);
    expect(error.messages).toEqual(['bad']);
  });

  test('NoAccess uses fixed 401 message', () => {
    const error = new NoAccess();

    expect(error.code).toBe(401);
    expect(error.message).toBe(
      'Не удалось определить пользователя. Обновите страницу и попробуйте снова.',
    );
  });

  test('NotFoundError uses status 404', () => {
    const error = new NotFoundError('Товар не найден');

    expect(error.code).toBe(404);
    expect(error.message).toBe('Товар не найден');
  });

  test('ServerError uses status 500', () => {
    const error = new ServerError(['oops']);

    expect(error.code).toBe(500);
    expect(error.messages).toEqual(['oops']);
  });
});

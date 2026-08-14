import { afterEach, beforeEach, describe, expect, rs, test } from '@rstest/core';

import { createAxiosMock } from '@test/mockAxios';

const axiosMock = createAxiosMock();

rs.mock('@shared/config', () => ({
  API_BASE_URL: 'https://api.example.com/api/v1',
  USER_ID_STORAGE_KEY: 'queue-service:user-id',
}));

rs.mock('axios', () => ({
  default: {
    create: axiosMock.create,
  },
}));

const { default: HttpClient } = await import('./HttpClient');

class TestClient extends HttpClient {
  constructor() {
    super('test');
  }

  getRoot() {
    return this.get<{ ok: boolean }>({ uri: '/x' });
  }

  postRoot(data: unknown) {
    return this.post({ uri: '/x', data });
  }
}

describe('HttpClient', () => {
  beforeEach(() => {
    localStorage.clear();
    axiosMock.get.mockReset();
    axiosMock.post.mockReset();
    axiosMock.create.mockClear();
    axiosMock.requestUse.mockClear();
    axiosMock.responseUse.mockClear();
  });

  afterEach(() => {
    localStorage.clear();
  });

  test('creates axios instance with joined baseURL', () => {
    new TestClient();

    expect(axiosMock.create).toHaveBeenCalledWith(
      expect.objectContaining({
        baseURL: 'https://api.example.com/api/v1/test',
        withCredentials: true,
      }),
    );
  });

  test('returns response data and sets X-User-Id', async () => {
    localStorage.setItem('queue-service:user-id', 'user-1');
    axiosMock.get.mockResolvedValue({ data: { ok: true } });

    const client = new TestClient();
    const requestInterceptor = axiosMock.requestUse.mock.calls[0][0] as (config: {
      headers: { set: (key: string, value: string) => void };
    }) => unknown;
    const headers = { set: rs.fn() };

    requestInterceptor({ headers });
    expect(headers.set).toHaveBeenCalledWith('X-User-Id', 'user-1');

    await expect(client.getRoot()).resolves.toEqual({ ok: true });
    expect(axiosMock.get).toHaveBeenCalledWith('/x', undefined);
  });

  test('maps response errors to typed HttpError subclasses', async () => {
    new TestClient();
    const onRejected = axiosMock.responseUse.mock.calls[0][1] as (error: unknown) => Promise<never>;

    await expect(
      onRejected({ response: { status: 400, data: { message: 'bad' } } }),
    ).rejects.toMatchObject({ code: 400, message: 'bad' });
    await expect(onRejected({ response: { status: 401 } })).rejects.toMatchObject({
      code: 401,
      message: 'Не удалось определить пользователя. Обновите страницу и попробуйте снова.',
    });
    await expect(
      onRejected({ response: { status: 404, data: { message: 'missing' } } }),
    ).rejects.toMatchObject({ code: 404, message: 'missing' });
    await expect(
      onRejected({ response: { status: 409, data: { status: 'SOLD_OUT' } } }),
    ).rejects.toMatchObject({
      code: 409,
      message: 'Товара больше нет',
    });
    await expect(
      onRejected({
        response: { status: 409, data: { error: 'queue_limit_reached', limit: 5 } },
      }),
    ).rejects.toMatchObject({
      code: 409,
      message:
        'Нельзя стоять больше чем в 5 очередях одновременно. Выйдите из одной, чтобы встать в новую.',
    });
    await expect(
      onRejected({ response: { status: 500, data: { message: 'boom' } } }),
    ).rejects.toMatchObject({ code: 500, message: 'boom' });
    await expect(
      onRejected({ response: { status: 418, data: { message: 'teapot' } } }),
    ).rejects.toMatchObject({ code: 418, message: 'teapot' });
  });
});

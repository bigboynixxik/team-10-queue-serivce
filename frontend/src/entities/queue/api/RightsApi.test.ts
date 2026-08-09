import { beforeEach, describe, expect, rs, test } from '@rstest/core';

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

const { rightsApi } = await import('./RightsApi');

describe('RightsApi', () => {
  beforeEach(() => {
    axiosMock.get.mockReset();
  });

  test('validate accepts valid:true response', async () => {
    axiosMock.get.mockResolvedValue({ data: { valid: true } });

    await expect(rightsApi.validate('token-1')).resolves.toBeUndefined();
    expect(axiosMock.get).toHaveBeenCalledWith('/token-1', undefined);
  });

  test('validate rejects invalid payload', async () => {
    axiosMock.get.mockResolvedValue({ data: { valid: false } });

    await expect(rightsApi.validate('token-1')).rejects.toThrow();
  });
});

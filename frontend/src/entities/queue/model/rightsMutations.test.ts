import { describe, expect, rs, test } from '@rstest/core';

const validate = rs.fn();

rs.mock('../api/RightsApi', () => ({
  rightsApi: { validate },
}));

const { rightsMutations } = await import('./rightsMutations');

describe('rightsMutations', () => {
  test('validate forwards token to rightsApi', async () => {
    validate.mockResolvedValue(undefined);
    const options = rightsMutations.validate();

    await options.mutationFn?.('token-1', {} as never);
    expect(validate).toHaveBeenCalledWith('token-1');
  });
});

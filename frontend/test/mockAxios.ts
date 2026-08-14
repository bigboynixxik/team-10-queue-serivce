import { rs } from '@rstest/core';

export const createAxiosMock = () => {
  const get = rs.fn();
  const post = rs.fn();
  const put = rs.fn();
  const patch = rs.fn();
  const del = rs.fn();
  const requestUse = rs.fn();
  const responseUse = rs.fn();
  const create = rs.fn(() => ({
    get,
    post,
    put,
    patch,
    delete: del,
    interceptors: {
      request: { use: requestUse },
      response: { use: responseUse },
    },
  }));

  return { get, post, put, patch, del, requestUse, responseUse, create };
};

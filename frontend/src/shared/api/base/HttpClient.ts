import axios, {
  type AxiosError,
  type AxiosInstance,
  type AxiosRequestConfig,
  type AxiosResponse,
} from 'axios';

import { API_BASE_URL, USER_ID_STORAGE_KEY } from '@shared/config';

import BadRequest from './error/BadRequest';
import { conflictMessage } from './error/conflictMessage';
import { getApiErrorMessages } from './error/get-api-error-messages';
import HttpError from './error/HttpError';
import NoAccess from './error/NoAccess';
import NotFoundError from './error/NotFoundError';
import ServerError from './error/ServerError';

type HttpClientOptions = {
  baseURL: string;
  headers?: Record<string, string>;
};

type HttpClientRequest<Data = unknown> = {
  uri?: string;
  data?: Data;
  config?: AxiosRequestConfig;
};

const joinUrlParts = (...parts: Array<string | undefined>): string =>
  parts
    .filter((part): part is string => Boolean(part))
    .map((part, index) =>
      index === 0 ? part.replace(/\/+$/g, '') : part.replace(/^\/+|\/+$/g, ''),
    )
    .filter(Boolean)
    .join('/');

abstract class HttpClient {
  private readonly instance: AxiosInstance;
  private localUri: string;

  constructor(localUri = '', options?: HttpClientOptions) {
    this.localUri = localUri;
    this.instance = axios.create({
      baseURL: options?.baseURL ?? joinUrlParts(API_BASE_URL, localUri),
      headers: options?.headers,
      withCredentials: true,
    });

    this.instance.interceptors.response.use(
      (response: AxiosResponse) => response,
      (error: AxiosError) => this.handleError(error),
    );
    this.instance.interceptors.request.use((config) => {
      const userId = localStorage.getItem(USER_ID_STORAGE_KEY);

      if (userId) config.headers.set('X-User-Id', userId);

      return config;
    });
  }

  protected get uri(): string {
    return this.localUri;
  }

  protected async get<Response = unknown>({
    uri = this.uri,
    config,
  }: HttpClientRequest): Promise<Response> {
    return this.instance
      .get<Response>(uri, config)
      .then((response: AxiosResponse<Response>) => response.data);
  }

  protected async post<Data = unknown, Response = unknown>({
    uri = this.uri,
    data,
    config,
  }: HttpClientRequest<Data>): Promise<Response> {
    return this.instance
      .post<Response>(uri, data, config)
      .then((response: AxiosResponse<Response>) => response.data);
  }

  protected async put<Data = unknown, Response = unknown>({
    uri = this.uri,
    data,
    config,
  }: HttpClientRequest<Data>): Promise<Response> {
    return this.instance
      .put<Response>(uri, data, config)
      .then((response: AxiosResponse<Response>) => response.data);
  }

  protected async patch<Data = unknown, Response = unknown>({
    uri = this.uri,
    data,
    config,
  }: HttpClientRequest<Data>): Promise<Response> {
    return this.instance
      .patch<Response>(uri, data, config)
      .then((response: AxiosResponse<Response>) => response.data);
  }

  protected async delete<Response = unknown>({
    uri = this.uri,
    config,
  }: HttpClientRequest): Promise<Response> {
    return this.instance
      .delete<Response>(uri, config)
      .then((response: AxiosResponse<Response>) => response.data);
  }

  private handleError(error: AxiosError): Promise<never> {
    const status = error.response?.status ?? error.status;
    const messages = getApiErrorMessages(error, error.message);

    switch (status) {
      case 400:
        return Promise.reject(new BadRequest(messages));
      case 401:
        return Promise.reject(new NoAccess());
      case 404:
        return Promise.reject(new NotFoundError(messages));
      case 409:
        return Promise.reject(
          new HttpError(status, conflictMessage(error.response?.data, messages)),
        );
      case 500:
        return Promise.reject(new ServerError(messages));
      default:
        return Promise.reject(new HttpError(status ?? 500, messages));
    }
  }
}

export default HttpClient;

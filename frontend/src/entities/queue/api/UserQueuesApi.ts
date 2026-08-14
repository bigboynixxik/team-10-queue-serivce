import { HttpClient } from '@shared/api';

import { type UserQueue, UserQueuesSchema } from './type';

class UserQueuesApi extends HttpClient {
  constructor() {
    super('me/queues');
  }

  public async getAll(): Promise<UserQueue[]> {
    // uri пустой: baseURL уже /me/queues, иначе получится /me/queues/me/queues
    return UserQueuesSchema.parse(await this.get<unknown>({ uri: '' }));
  }
}

export const userQueuesApi = new UserQueuesApi();

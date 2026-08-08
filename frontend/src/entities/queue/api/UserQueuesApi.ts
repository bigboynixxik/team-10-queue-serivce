import { HttpClient } from '@shared/api';

import { type UserQueue, UserQueuesSchema } from './type';

class UserQueuesApi extends HttpClient {
  constructor() {
    super('me/queues');
  }

  public async getAll(): Promise<UserQueue[]> {
    return UserQueuesSchema.parse(await super.get<unknown>({}));
  }
}

export const userQueuesApi = new UserQueuesApi();

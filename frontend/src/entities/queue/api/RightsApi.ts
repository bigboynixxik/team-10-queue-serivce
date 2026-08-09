import { HttpClient } from '@shared/api';

import { RightValidationSchema } from './type';

class RightsApi extends HttpClient {
  constructor() {
    super('rights');
  }

  public async validate(token: string): Promise<void> {
    RightValidationSchema.parse(await this.get<unknown>({ uri: `/${token}` }));
  }
}

export const rightsApi = new RightsApi();

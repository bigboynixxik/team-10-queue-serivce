import HttpError from './HttpError';

class NoAccess extends HttpError {
  constructor() {
    super(401, 'Not Access use Refresh or JWT');
  }
}

export default NoAccess;

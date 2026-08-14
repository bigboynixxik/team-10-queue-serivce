import HttpError from './HttpError';

class NoAccess extends HttpError {
  constructor() {
    super(401, 'Не удалось определить пользователя. Обновите страницу и попробуйте снова.');
  }
}

export default NoAccess;

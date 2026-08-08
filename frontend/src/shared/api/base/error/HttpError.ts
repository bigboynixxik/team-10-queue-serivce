const normalizeMessages = (messages: string | string[]): string[] => {
  const list = (Array.isArray(messages) ? messages : [messages])
    .map((message) => message.trim())
    .filter(Boolean);

  return list.length > 0 ? list : ['Произошла ошибка'];
};

class HttpError extends Error {
  public readonly messages: string[];

  constructor(
    public code: number,
    message: string | string[],
  ) {
    const messages = normalizeMessages(message);
    super(messages[0]);
    this.messages = messages;
    this.name = 'HttpError';
    Object.setPrototypeOf(this, HttpError.prototype);
  }
}

export default HttpError;

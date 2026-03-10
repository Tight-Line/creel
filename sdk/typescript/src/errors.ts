/**
 * Error thrown by the Creel client when an API request fails.
 */
export class CreelError extends Error {
  public readonly statusCode: number;
  public readonly body: unknown;

  constructor(statusCode: number, message: string, body?: unknown) {
    super(message);
    this.name = "CreelError";
    this.statusCode = statusCode;
    this.body = body;
  }
}

/**
 * Custom error class for API errors.
 * Used for type checking and error identification.
 */
export class ApiError extends Error {
  constructor(
    message: string,
    public status?: number,
    public code?: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

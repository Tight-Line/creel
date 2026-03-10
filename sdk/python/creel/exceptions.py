"""Custom exceptions for the Creel SDK."""


class CreelError(Exception):
    """Raised when the Creel API returns an error response.

    Attributes:
        status_code: HTTP status code from the server.
        message: Human-readable error message.
    """

    def __init__(self, status_code: int, message: str) -> None:
        self.status_code = status_code
        self.message = message
        super().__init__(f"HTTP {status_code}: {message}")

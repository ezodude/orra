from asyncio import CancelledError

from fastapi import Request
from fastapi.responses import JSONResponse
from starlette.middleware.base import BaseHTTPMiddleware, RequestResponseEndpoint


class CancelledErrorMiddleware(BaseHTTPMiddleware):
    async def dispatch(
            self, request: Request, call_next: RequestResponseEndpoint
    ):
        try:
            response = await call_next(request)
        except CancelledError:
            # Handle the CancelledError here
            print("\nAn operation was cancelled...")
            response = JSONResponse({"detail": "CancelledError: An operation was cancelled."}, status_code=400)
        return response


class KeyboardInterruptMiddleware(BaseHTTPMiddleware):
    async def dispatch(
            self, request: Request, call_next: RequestResponseEndpoint
    ):
        try:
            response = await call_next(request)
        except KeyboardInterrupt:
            # Handle the KeyboardInterrupt here
            print("\nCtrl+C was pressed, exiting...")
            response = JSONResponse({"detail": "KeyboardInterrupt: Ctrl+C was pressed."}, status_code=400)
        return response


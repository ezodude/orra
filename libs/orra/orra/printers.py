from typing import Protocol

from fastapi import FastAPI
from pydantic import BaseModel


class Printer(Protocol):
    def print(self, message: str) -> None:
        ...


class NullPrinter:
    def print(self, message: str) -> None:
        pass


def print_pydantic_models_from(app: FastAPI):
    for route in app.routes:
        if hasattr(route, "endpoint"):
            print(f"Route: {route.path}")
            for param in route.endpoint.__annotations__.values():
                if issubclass(param, BaseModel):
                    print(f"  Pydantic model: {param.__name__}")
                    for field_name, field_value in param.__annotations__.items():
                        print(f"    Field: {field_name}, Type: {field_value}")

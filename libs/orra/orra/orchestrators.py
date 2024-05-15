from pydoc import Doc
from typing import Type, Any, TypedDict, Callable, Annotated

from fastapi import FastAPI
from pydantic import BaseModel


def _create_typed_dict(name: str, fields: dict[str, Any]) -> Any:
    """
        Create a TypedDict from a dictionary of fields
    """
    return TypedDict(name, fields)


def _create_response_model(typed_dict: Type[Any]) -> Type[BaseModel]:
    """
        Create a Pydantic model from a TypedDict
    """
    class Model(BaseModel):
        __annotations__ = typed_dict.__annotations__

    return Model


class StepResponse(BaseModel):
    name: str
    description: str | None = None


class Orra(FastAPI):
    def __init__(self, state_def=None, **extra: Annotated[
        Any,
        Doc(),
    ]):
        super().__init__(**extra)
        if state_def is None:
            state_def = {}

        self._workflow = ""
        self._workflow_invoked = False
        self._StateDict = _create_typed_dict("StateDict", state_def)

    def step(self, func: Callable) -> Callable:
        print(f"decorated with step: {func.__name__}")
        self._workflow = f"{self._workflow} | {func.__name__}" if len(self._workflow) > 0 else func.__name__

        return self.post(path=f"/{func.__name__}", response_model=None)(func)

    # def after(self, act: str) -> Callable:
    #     def decorator(func: Callable) -> Callable:
    #         print(f"decorated {func.__name__} with activity: {act}")
    #         self._workflow = f"{self._workflow} | {func.__name__}"
    #         return func
    #     return decorator

    def run(self) -> None:
        print("running: ", self._workflow)

    def stop(self) -> None:
        print("stopping: ", self._workflow)

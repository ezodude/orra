from pydoc import Doc
from typing import Type, Any, TypedDict, Callable, Annotated

from fastapi import FastAPI
from pydantic import BaseModel
from langgraph.graph import StateGraph, END


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

        self._StateDict = _create_typed_dict("StateDict", state_def)
        self._workflow = StateGraph(self._StateDict)
        self._steps = []
        self._compiled_workflow = None

    def step(self, func: Callable) -> Callable:
        print(f"decorated with step: {func.__name__}")
        self._orchestrate(func)
        return self.post(path=f"/{func.__name__}", response_model=_create_response_model(self._StateDict))(func)

    def _orchestrate(self, func: Callable):
        self._workflow.add_node(func.__name__, func)
        self._steps.append(func.__name__)

    # def after(self, act: str) -> Callable:
    #     def decorator(func: Callable) -> Callable:
    #         print(f"decorated {func.__name__} with activity: {act}")
    #         self._workflow = f"{self._workflow} | {func.__name__}"
    #         return func
    #     return decorator

    def local(self) -> None:
        for i in range(len(self._steps) - 1):
            print(self._steps[i], self._steps[i + 1])
            self._workflow.add_edge(self._steps[i], self._steps[i + 1])

        if len(self._steps) > 1:
            self._workflow.set_entry_point(self._steps[0])
            self._workflow.add_edge(self._steps[-1], END)

        print(self._workflow.nodes)

        self._compiled_workflow = self._workflow.compile()
        self._compiled_workflow.invoke({})

    def run(self) -> None:
        if len(self._steps) > 1:
            self._workflow.add_edge(self._steps[-1], END)

        self._compiled_workflow = self._workflow.compile()
        self.post(path=f"/workflow", response_model=None)(self._compiled_workflow.invoke)({})


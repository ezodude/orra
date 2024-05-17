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
        self._register(func)
        return self.post(path=f"/{func.__name__}", response_model=_create_response_model(self._StateDict))(func)

    # def after(self, act: str) -> Callable:
    #     def decorator(func: Callable) -> Callable:
    #         print(f"decorated {func.__name__} with activity: {act}")
    #         self._workflow = f"{self._workflow} | {func.__name__}"
    #         return func
    #     return decorator

    def local(self) -> None:
        self._compiled_workflow = self._compile(self._workflow, self._steps)
        self._compiled_workflow.invoke({})

    def run(self) -> None:
        self._compiled_workflow = self._compile(self._workflow, self._steps)
        self.post(path=f"/workflow", response_model=None)(self._compiled_workflow.invoke)({})

    def _register(self, func: Callable):
        self._workflow.add_node(func.__name__, func)
        self._steps.append(func.__name__)

    @staticmethod
    def _compile(workflow, steps):
        for i in range(len(steps) - 1):
            print(steps[i], steps[i + 1])
            workflow.add_edge(steps[i], steps[i + 1])

        if len(steps) > 1:
            workflow.set_entry_point(steps[0])
            workflow.add_edge(steps[-1], END)

        return workflow.compile()


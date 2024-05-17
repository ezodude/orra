from typing import Any, Callable

import fastapi
from langgraph.graph import StateGraph, END

from .printers import print_pydantic_models_from
from .typing_dynamic import create_typed_dict, create_response_model


class Orra:
    def __init__(self, schema: dict[str, Any] = None):
        if schema is None:
            schema = {}

        self._steps_app = fastapi.FastAPI()
        self._steps = []
        self._StateDict = create_typed_dict("StateDict", schema)
        self._StepResponseModel = create_response_model(self._StateDict)
        self._workflow = StateGraph(self._StateDict)
        self._compiled_workflow = None

    def step(self, func: Callable) -> Callable:
        self._register(func)
        response_model = self._StepResponseModel

        @self._steps_app.post(f"/{func.__name__}")
        async def wrap_endpoint(v: response_model):
            func(v.dict())

        return func

    # def after(self, act: str) -> Callable:
    #     def decorator(func: Callable) -> Callable:
    #         print(f"decorated {func.__name__} with activity: {act}")
    #         self._workflow = f"{self._workflow} | {func.__name__}"
    #         return func
    #     return decorator

    def run(self) -> Callable:
        self._compiled_workflow = self._compile(self._workflow, self._steps)

        @self._steps_app.post(f"/workflow")
        async def wrap_workflow():
            self._compiled_workflow.invoke({})

        print_pydantic_models_from(self._steps_app)
        return self._steps_app

    def execute(self) -> None:
        self._compiled_workflow = self._compile(self._workflow, self._steps)
        self._compiled_workflow.invoke({})

    def _register(self, func: Callable):
        self._workflow.add_node(func.__name__, func)
        self._steps.append(func.__name__)

    @staticmethod
    def _compile(workflow, steps):
        parts = ""
        for i in range(len(steps) - 1):
            parts = f"{steps[i]} -> {steps[i + 1]}" if parts == "" else f"{parts} -> {steps[i + 1]}"
            workflow.add_edge(steps[i], steps[i + 1])

        if len(steps) > 1:
            workflow.set_entry_point(steps[0])
            workflow.add_edge(steps[-1], END)

        print("compiling workflow steps:", parts)
        return workflow.compile()


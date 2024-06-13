import asyncio
import inspect
from typing import Any, Callable

import fastapi
from langgraph.graph import StateGraph, END

from .printers import Printer, NullPrinter
from .signals import KeyboardInterruptMiddleware, CancelledErrorMiddleware
from .typing_dynamic import create_typed_dict, create_response_model


class Orra:
    def __init__(self, schema: dict[str, Any] = None):
        if schema is None:
            schema = {}

        self._steps_app = fastapi.FastAPI()
        # noinspection PyTypeChecker
        self._steps_app.add_middleware(KeyboardInterruptMiddleware)
        # noinspection PyTypeChecker
        self._steps_app.add_middleware(CancelledErrorMiddleware)

        self._steps = []
        self._StateDict = create_typed_dict("StateDict", schema)
        self._StepResponseModel = create_response_model(self._StateDict)
        self._flow = StateGraph(self._StateDict)
        self._compiled_flow = None

    def step(self, func: Callable) -> Callable:
        self._register(func)
        response_model = self._StepResponseModel

        @self._steps_app.post(f"/flow/{func.__name__}")
        async def wrap_endpoint(v: response_model):
            if inspect.iscoroutinefunction(func):
                return await func(v.dict())
            else:
                return func(v.dict())

        return func

    def compile(self, printer: Printer = NullPrinter(), debug: bool = False) -> Callable:
        if debug:
            printer.print("Initialising \[debug] mode...Done!")

        msg = "Compiling Orra application flow..."
        self._compiled_flow = self._compile(self._flow, self._steps)
        msg = f"{msg} Done!"
        printer.print(msg)

        printer.print("Prepared Orra application step endpoints...Done!")

        msg = "Preparing Orra application flow endpoint..."
        response_model = self._StepResponseModel

        @self._steps_app.post(f"/flow")
        async def wrap_flow(v: response_model):
            return self._compiled_flow.invoke(v.dict(), debug=debug)

        msg = f"{msg} Done!"
        printer.print(msg)

        # print_pydantic_models_from(self._steps_app)

        return self._steps_app

    def execute(self) -> None:
        self._compiled_flow = self._compile(self._flow, self._steps)
        self._compiled_flow.invoke({})

    def _register(self, func: Callable):
        if inspect.iscoroutinefunction(func):
            def wrap_async(*args, **kwargs):
                return asyncio.run(func(*args, **kwargs))
            executable_func = wrap_async
        else:
            executable_func = func

        self._flow.add_node(func.__name__, executable_func)
        self._steps.append(func.__name__)

    @staticmethod
    def _compile(flow, steps):
        parts = ""
        for i in range(len(steps) - 1):
            parts = f"{steps[i]} -> {steps[i + 1]}" if parts == "" else f"{parts} -> {steps[i + 1]}"
            flow.add_edge(steps[i], steps[i + 1])

        if len(steps) > 1:
            flow.set_entry_point(steps[0])
            flow.add_edge(steps[-1], END)

        # print("compiling flow steps:", parts)
        return flow.compile()

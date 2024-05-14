from typing import Callable

from typing import Any, TypedDict


def _create_typed_dict(name: str, fields: dict[str, Any]) -> Any:
    return TypedDict(name, fields)


class Orra:
    def __init__(self, state_def=None):
        if state_def is None:
            state_def = {}

        self._workflow = ""
        self._workflow_invoked = False
        self._StateDict = _create_typed_dict("StateDict", state_def)

    def step(self, func: Callable) -> Callable:
        print(f"decorated with step: {func.__name__}")
        self._workflow = f"{self._workflow} | {func.__name__}" if len(self._workflow) > 0 else func.__name__
        return func

    # def after(self, act: str) -> Callable:
    #     def decorator(func: Callable) -> Callable:
    #         print(f"decorated {func.__name__} with activity: {act}")
    #         self._workflow = f"{self._workflow} | {func.__name__}"
    #         return func
    #     return decorator

    def run(self) -> None:
        self._workflow_invoked = True
        print("running: ", self._workflow)

    def stop(self) -> None:
        self._workflow_invoked = False
        print("stopping: ", self._workflow)

    def __call__(self) -> None:
        if self._workflow_invoked:
            print("Workflow already invoked")
            return

        print(f"{self._workflow}.invoke()")
        pass

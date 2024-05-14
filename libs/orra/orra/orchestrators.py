from typing import Callable


class Orra:
    def __init__(self):
        self._workflow = ""
        self._workflow_invoked = False
        self._state = {
            "tracked_issues": ["issue1", "issue2"],
        }

    def root(self, func: Callable) -> Callable:
        print(f"decorated {func.__name__} as root")
        self._workflow = func.__name__
        return func

    def after(self, act: str) -> Callable:
        def decorator(func: Callable) -> Callable:
            print(f"decorated {func.__name__} with activity: {act}")
            self._workflow = f"{self._workflow} | {func.__name__}"
            return func
        return decorator

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

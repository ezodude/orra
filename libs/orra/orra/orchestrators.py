import functools
from typing import Callable


class Orra:
    def __init__(self):
        self._workflow = ""
        self._state = {
            "tracked_issues": ["issue1", "issue2"],
        }

    def root(self, func: Callable) -> Callable:
        @functools.wraps(func)
        def wrapper_root(*args, **kwargs):
            print(f"decorated {func.__name__} as root")
            self._workflow = func.__name__
            with_state = {**kwargs, "state": self._state}
            return func(*args, **with_state)

        return wrapper_root

    def after(self, activity: str) -> Callable:
        def after_decorator(func: Callable) -> Callable:
            @functools.wraps(func)
            def wrapper_after(*args, **kwargs):
                print(f"decorated {func.__name__} with activity: {activity}")
                self._workflow = f"{self._workflow} | {func.__name__}"
                with_state = {**kwargs, "state": self._state}
                return func(*args, **with_state)

            return wrapper_after

        return after_decorator

    def __call__(self) -> None:
        pass

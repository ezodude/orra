import functools
from typing import Callable


class Orra:
    def __init__(self):
        self._workflow = ""
        self._state = {}

    def root(self, func: Callable) -> Callable:
        @functools.wraps(func)
        def wrapper_root(*args, **kwargs):
            print(f"decorated {func.__name__} as root")
            self._workflow = func.__name__
            return func(*args, **kwargs)
        return wrapper_root

    def after(self, activity: str) -> Callable:
        def after_decorator(func: Callable) -> Callable:
            @functools.wraps(func)
            def wrapper_after(*args, **kwargs):
                print(f"decorated {func.__name__} with activity: {activity}")
                self._workflow = f"{self._workflow} | {func.__name__}"
                return func(*args, **kwargs)
            return wrapper_after
        return after_decorator

    def __call__(self) -> None:
        pass

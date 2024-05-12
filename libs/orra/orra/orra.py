import functools
from typing import Callable


def root(func: Callable) -> Callable:
    @functools.wraps(func)
    def wrapper_root(*args, **kwargs):
        print(f"decorated {func.__name__} as root")
        return func(*args, **kwargs)
    return wrapper_root


def after(activity: str) -> Callable:
    def after_decorator(func: Callable) -> Callable:
        @functools.wraps(func)
        def wrapper_after(*args, **kwargs):
            print(f"decorated {func.__name__} with activity: {activity}")
            return func(*args, **kwargs)
        return wrapper_after
    return after_decorator

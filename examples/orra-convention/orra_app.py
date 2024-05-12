from typing import Any

import activities
from orra import Orra

app = Orra()


@app.root
def check_issues() -> Any:
    return activities.check_issues()


@app.after(activity='check_issues')
def research() -> Any:
    return activities.research()


@app.after(activity='research')
def author_workarounds() -> Any:
    return activities.author_workarounds()


@app.after(activity='research')
def resolve() -> Any:
    return activities



from typing import Any

import activities
from orra import Orra

app = Orra()


@app.root
def check_issues(state: dict) -> Any:
    print('decorated research state', state)
    return activities.check_issues()


@app.after(activity='check_issues')
def research(state: dict) -> Any:
    return activities.research()


@app.after(activity='research')
def author_workarounds(state: dict) -> Any:
    return activities.author_workarounds()


@app.after(activity='research')
def resolve(state: dict) -> Any:
    return activities



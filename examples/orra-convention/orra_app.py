from typing import Any

import activities
from orra import Orra

app = Orra()


@app.root
def check_issues(state: dict) -> Any:
    print('decorated research state', state)
    return activities.check_issues()


@app.after(act='check_issues')
def research(state: dict) -> Any:
    print('decorated research state', state)
    return activities.research()


@app.after(act='research')
def author_workarounds(state: dict) -> Any:
    print('decorated author_workarounds state', state)
    return activities.author_workarounds()


@app.after(act='research')
def resolve(state: dict) -> Any:
    print('decorated resolve state', state)
    return activities



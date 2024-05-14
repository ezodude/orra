from typing import Any

import activities
from orra import Orra

app = Orra(
    state_def={
        "tracked_issues": list[str]
    }
)


@app.step
def check_issues(state: dict) -> Any:
    print('decorated research state', state)
    return activities.check_issues()


@app.step
def research(state: dict) -> Any:
    print('decorated research state', state)
    return activities.research()


@app.step
def author_workarounds(state: dict) -> Any:
    print('decorated author_workarounds state', state)
    return activities.author_workarounds()


@app.step
def resolve(state: dict) -> Any:
    print('decorated resolve state', state)
    return activities

from typing import Any

import activities
from orra import Orra

app = Orra(
    state_def={
        "tracked_issues": list[dict] | None,
        "researched": list[dict] | None,
    }
)


@app.step
def check_issues(state) -> Any:
    print('decorated check_issues')
    activities.check_issues()
    return state


@app.step
def research(state) -> Any:
    print('decorated research')
    activities.research()
    return state


@app.step
def author_workarounds(state) -> Any:
    print('decorated author_workarounds')
    activities.author_workarounds()
    return state


@app.step
def resolve(state) -> Any:
    print('decorated resolve')
    activities.resolve()
    return state

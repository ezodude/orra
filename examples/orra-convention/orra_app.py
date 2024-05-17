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
    result = activities.check_issues()
    return {
        **state,
        "tracked_issues": result
    }


@app.step
def research(state) -> Any:
    print('decorated research', state)
    result = activities.research(state["tracked_issues"])
    return {
        **state,
        "researched": result
    }


@app.step
def author_workarounds(state) -> Any:
    print('decorated author_workarounds')
    activities.author_workarounds()
    return state


@app.step
def resolve(state) -> Any:
    print('decorated resolve', state)
    activities.resolve()
    return state

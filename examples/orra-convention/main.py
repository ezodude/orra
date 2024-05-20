from typing import Any
from typing import Optional, List, Dict

from orra import Orra

import steps

app = Orra(
    schema={
        "tracked_issues": Optional[List[Dict]],
        "researched": Optional[List[Dict]],
    }
)


@app.step
def check_issues(state: dict) -> Any:
    print('decorated check_issues')
    result = steps.check_issues()
    return {
        **state,
        "tracked_issues": result
    }


@app.step
def research(state: dict) -> Any:
    print('decorated research', state)
    result = steps.research(state["tracked_issues"])
    return {
        **state,
        "researched": result
    }


@app.step
def author_workarounds(state: dict) -> Any:
    print('decorated author_workarounds')
    steps.author_workarounds()
    return state


@app.step
def resolve(state: dict) -> Any:
    print('decorated resolve', state)
    steps.resolve()
    return state

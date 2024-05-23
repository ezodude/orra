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
def discover_dependencies(state: dict) -> Any:
    print('decorated discover_dependencies')
    result = steps.discover_dependencies()
    return {
        **state,
        "tracked_issues": result
    }


@app.step
def research_updates(state: dict) -> Any:
    print('decorated research_updates', state)
    result = steps.research_updates(state["tracked_issues"])
    return {
        **state,
        "researched": result
    }


@app.step
def draft_prs(state: dict) -> Any:
    print('decorated draft_prs')
    steps.draft_prs()
    return state


@app.step
def submit_prs(state: dict) -> Any:
    print('decorated submit_prs', state)
    steps.submit_prs()
    return state

from typing import Any

import activities
from orra import Orra

app = Orra(
    state_def={
        "tracked_issues": list[dict],
        "researched": list[dict],
    }
)


@app.step
def check_issues() -> Any:
    print('decorated check_issues')
    return activities.check_issues()


@app.step
def research() -> Any:
    print('decorated research')
    return activities.research()


@app.step
def author_workarounds() -> Any:
    print('decorated author_workarounds')
    return activities.author_workarounds()


@app.step
def resolve() -> Any:
    print('decorated resolve')
    return activities.resolve()

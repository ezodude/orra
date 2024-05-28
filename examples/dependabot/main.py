import asyncio
from typing import Any
from typing import Optional, List, Dict

from dotenv import load_dotenv

load_dotenv()

from orra import Orra

import steps

app = Orra(
    schema={
        "dependencies": Optional[List[Dict]],
        "researched": Optional[List[Dict]],
        "drafted": Optional[List[Dict]],
        "submitted": Optional[List[str]]
    }
)


@app.step
def discover_dependencies(state: dict) -> Any:
    result = steps.discover_dependencies()
    return {
        **state,
        "dependencies": result
    }


@app.step
def research_updates(state: dict) -> Any:
    result = [asyncio.run(steps.research_update(dependency)) for dependency in state['dependencies']]
    return {
        **state,
        "researched": result
    }


@app.step
def draft_issues(state: dict) -> Any:
    result = steps.run_draft_issues(state['researched'])
    return {
        **state,
        "drafted": result
    }


@app.step
def submit_issues(state: dict) -> Any:
    commands = steps.submit_issues(state['drafted'])
    return {
        **state,
        "submitted": commands
    }

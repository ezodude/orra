import asyncio
from typing import Any
from typing import Optional, List, Dict

from dotenv import load_dotenv

load_dotenv()

from orra import Orra

import steps

# Initialise an Orra instance to create a Dependabot backend.

# A backend is created using a flow that is constructed as a series of steps.
# Each step has its own API endpoint. They that are orchestrated into a flow and
# later executed **in the order they are defined**.
# The `@app.step` decorator is used to define a step.

# All steps share state. Orra requires you to declare the schema used by the state object.
# This schema validates the state object and provides type hints to the steps.
# Every step must return a new state object.
app = Orra(
    schema={
        "dependencies": Optional[List[Dict]],
        "researched": Optional[List[Dict]],
        "drafted": Optional[List[Dict]],
        "submitted": Optional[List[str]]
    }
)


# The `discover_dependencies` step discovers dependencies that require an update.
# A POST API endpoint is created for this step at path: `/flow/discover_dependencies`.
# This simplifies testing and integration checks.
@app.step
def discover_dependencies(state: dict) -> Any:
    result = steps.discover_dependencies()
    return {
        **state,
        "dependencies": result
    }


# The `research_updates` step researches dependency updates using the GPT-Research agent.
# A POST API endpoint is created for this step at path: `/flow/research_updates`.
# This simplifies testing and integration checks.
@app.step
async def research_updates(state: dict) -> Any:
    tasks = [steps.research_update(dependency) for dependency in state['dependencies']]
    result = await asyncio.gather(*tasks)
    return {
        **state,
        "researched": result
    }


# The `draft_issues` step drafts GitHub issues based on dependency research using a CrewAI agent crew.
# A POST API endpoint is created for this step at path: `/flow/draft_issues`.
# This simplifies testing and integration checks.
@app.step
def draft_issues(state: dict) -> Any:
    result = steps.run_draft_issues(state['researched'])
    return {
        **state,
        "drafted": result
    }


# The `submit_issues` step generates API calls to simulate submitting the drafted GitHub issues.
# A POST API endpoint is created for this step at path: `/flow/submit_issues`.
# This simplifies testing and integration checks.
@app.step
def submit_issues(state: dict) -> Any:
    commands = steps.submit_issues(state['drafted'])
    return {
        **state,
        "submitted": commands
    }

# **** Use the CLI to run your backend. This implicitly creates a POST API endpoint at path: `/flow`. ****
# *** Call this endpoint to execute the whole Dependabot flow. ***

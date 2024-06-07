import asyncio
from typing import Any
from typing import Optional, List, Dict

from dotenv import load_dotenv

load_dotenv()

from orra import Orra

import steps

# Initialise an Orra app to orchestrate the Dependabot as a service-based workflow.

# The workflow is made up of a series of steps that are orchestrated
# and later executed **in the order they are defined**.
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
    },
    debug=True
)


# The `discover_dependencies` step discovers dependencies that require an update.
# A POST HTTP endpoint is created for this step at path: `/workflow/discover_dependencies`.
# This simplifies testing and integration checks.
@app.step
def discover_dependencies(state: dict) -> Any:
    result = steps.discover_dependencies()
    return {
        **state,
        "dependencies": result
    }


# The `research_updates` step researches dependency updates using the GPT-Research agent.
# A POST HTTP endpoint is created for this step at path: `/workflow/research_updates`.
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
# A POST HTTP endpoint is created for this step at path: `/workflow/draft_issues`.
# This simplifies testing and integration checks.
@app.step
def draft_issues(state: dict) -> Any:
    result = steps.run_draft_issues(state['researched'])
    return {
        **state,
        "drafted": result
    }


# The `submit_issues` step generates API calls to simulate submitting the drafted GitHub issues.
# A POST HTTP endpoint is created for this step at path: `/workflow/submit_issues`.
# This simplifies testing and integration checks.
@app.step
def submit_issues(state: dict) -> Any:
    commands = steps.submit_issues(state['drafted'])
    return {
        **state,
        "submitted": commands
    }

# **** Use the CLI to run the app. This creates a POST HTTP endpoint at path: `/workflow`. ****
# *** Call this endpoint to execute the Dependabot workflow. ***

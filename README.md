# 🪡 orra

🦸 Use an opinionated workflow to orchestrate and deploy agents rapidly - with batteries included!

Orra helps you create reliable and deterministic multi-agent backed systems. It provides a simple and structured way to build multi-agent workflows, fine-tune and verify their reliability then deploy them.

It consists of a **Backend SDK**, a **Local Development Environment** with Agent specific **workflow tooling**, **integrations** and a **Cloud Platform** for automating deployments.

It works with purpose built Agents like [GPT Researcher](https://github.com/assafelovic/gpt-researcher) or Agent frameworks like [LangChain](https://python.langchain.com/v0.1/docs/modules/agents/), [CrewAI](http://crewai.com/) and more.

## In progress

- [ ] Local Development Environment

## We're just getting started

We're just getting started and are ironing out the details of a **Local Development Environment**.

Use the [Orra SDK](libs/orra) to create an app instance, then decorate any function with an `app.step` to create a workflow. The steps are then orchestrated by Orra to execute the workflow.

For example:

```python

from orra import Orra
import steps

app = Orra(
    schema={
        "dependencies": Optional[List[Dict]],
        "researched": Optional[List[Dict]],
        "drafted": Optional[List[Dict]],
        "submitted": Optional[List[str]]
    },
    debug=True
)


@app.step
def discover_dependencies(state: dict) -> Any:
    result = steps.do_something()
    return {
        **state,
        "dependencies": result
    }
...
...
# more steps
```

Using the [**Orra CLI**](libs/cli) you can run the workflow (in the root of your Orra project), this creates: 
- A set of API endpoints for each step in the workflow.
- A dedicated workflow API endpoint.
- A development server that runs the workflow.

You can then interact with the API endpoints to run the workflow (at `/workflow`), or run each step individually (e.g. `/workflow/step_name`).

```bash
% poetry run python -m orra_cli run
  ✔ Compiling Orra application workflow... Done!
  ✔ Prepared Orra application step endpoints...Done!
  ✔ Preparing Orra application workflow endpoint... Done!
  ✔ Starting Orra application... Done!

  Orra development server running!
  Your API is running at:     http://127.0.0.1:1430

INFO:     Started server process [21403]
INFO:     Waiting for application startup.
INFO:     Application startup complete.
INFO:     Server running on http://127.0.0.1:1430 (Press CTRL+C to quit)
```

## The Dependabot Example

The [Dependabot example](examples/dependabot) is a simple Orra project that builds a multi-agent based Dependabot to fetch a list of dependencies for a given repository. It then drafts GitHub issues for each dependency update.

The core steps are:
- `research_updates`: Researches updates for every discovered dependency requiring an update - using the [GPT Researcher Agent](https://github.com/assafelovic/gpt-researcher). 
- `draft_issues`: Reviews the updates and drafts GitHub issues for each update - using custom [CrewAI](http://crewai.com/) Agents.

It showcases how Orra uses convention to wire up a multi-agent backed system **using a heterogeneous set of agents**.

## Why Orra?

The developer experience for orchestrating multi-agents for reliable and repeatable workflows in production is still lacking.

Currently, a developer has to work close to the metal. They have to source and glue various libraries and tools for every project they create. Then revert into DevOPS mode to factor in cost monitoring, setting up any required prompt/model fine-tuning pipelines and finally deployment.

Also, adding reliability/eval checks require custom code that varies depending on the frameworks used. Agent reuse is another hurdle, especially sourcing and vetting agents to ensure they work as advertised.

Orra is here to take you to the next level. ⚡️⚡️


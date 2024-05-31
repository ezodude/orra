# Dependabot Example

This project simulates a Dependabot-like agent system built on Orra to draft GitHub issues for outdated dependencies.

## How it works

The project's workflow is orchestrated as steps in [main.py](main.py) file. The file is well documented and
showcases how Orra uses convention to wire up a multi-agent backed system.

Each orchestrated step is a function that can be a simple Python function or a more complex Agent.

## Agent steps

- [research_updates](steps/research_updates/main.py): Researches updates for every discovered dependency requiring an
  update - using
  the [GPT Researcher Agent](https://github.com/assafelovic/gpt-researcher).

- [draft_issues](steps/draft_issues/main.py): Reviews the updates and drafts GitHub issues for each update - using
  custom [CrewAI](https://github.com/joaomdmoura/crewAI) Agents.

## Running the project

**Requirements:**

- Clone the Orra repository.
- [Poetry installed](https://python-poetry.org/docs/#installation).
- [Open AI API](https://openai.com/api/) key.
- [Tavily API](https://docs.tavily.com/docs/tavily-api/langchain) key - they
  have [a generous Free tier](https://tavily.com/#pricing).

After cloning the Orra repository, navigate to the `examples/dependabot` directory:

1. Install the Dependabot project dependencies (installs Orra SDK and CLI as local packages):

```shell
poetry install
```

2. Set the required environment variables in a `.env` file:

```shell
cp .env.example .env

# Update OPENAI_API_KEY=<your_openai_api_key>
# Update TAVILY_API_KEY=<your_tavily_api_key>
```

3. Run the project:

```shell
poetry run python -m orra_cli run
```

4. Execute the Dependabot by sending a POST request to the `/workflow` endpoint:

```shell
curl -X POST \
  -H "Content-Type: application/json" \
  -d '{"dependencies": [], "researched": [], "drafted": [], "submitted": []}' \
  http://127.0.0.1:1430/workflow
```

7. Execute individual Dependabot steps by sending a POST request to the `/workflow/step_name` endpoint.
   E.g. to execute the `discover_dependencies` step, run the following command:

```shell
curl -X POST \
  -H "Content-Type: application/json" \
  -d '{"dependencies": [], "researched": [], "drafted": [], "submitted": []}' \
  http://127.0.0.1:1430/workflow/discover_dependencies
```

> **Note**:
> 
> Every step requires a different payload to execute successfully.
> E.g. `research_updates` requires a list of dependencies to research, `draft_issues` requires a list of researched
> dependencies to draft issues for, etc.


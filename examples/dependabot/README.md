# Dependabot Example

This is a simple Orra project that builds a multi-agent based Dependabot to fetch a list of dependencies for a given repository. It then drafts GitHub issues for each dependency update.

The core steps are:
- `research_updates`: Researches updates for every discovered dependency requiring an update - using the [GPT Researcher Agent](https://github.com/assafelovic/gpt-researcher).
- `draft_issues`: Reviews the updates and drafts GitHub issues for each update - using custom [CrewAI](https://github.com/joaomdmoura/crewAI) Agents.

It showcases how Orra uses convention to wire up a multi-agent backed system **using a heterogeneous set of agents**.

# 🪡 orra

🦸 Use an opinionated workflow to orchestrate and deploy LLM powered Multi-Agent systems rapidly - batteries
included!

Orra provides a **Python SDK** and a **Local Development Environment**. And soon, agentic workflow tooling,
integrations and a Cloud Platform for automating deployments, to develop reliable and
deterministic multi-agent systems.

## Bring your own agents

Using Orra, you can seamlessly integrate purpose-built agents
e.g. [GPT Researcher](https://github.com/assafelovic/gpt-researcher)
with custom agents built
with [LangChain](https://python.langchain.com/v0.1/docs/modules/agents/), [CrewAI](https://github.com/joaomdmoura/crewAI),
and more.

## Why Orra?

Orchestrating multi-agent LLM workflows is complex. Orra simplifies it by providing an open-source platform for
reliable, repeatable agent orchestration. 🚀 No more gluing libraries or custom code for cost monitoring, fine-tuning,
deployment, reliability checks, and agent vetting. Orra streamlines the entire process. ⚡️⚡️

## We're just getting started

We're still ironing out the details of our **Local Development Environment**.

You can try out the latest by installing a local version of Orra.

(See the [Dependabot example](examples/dependabot) for a detailed example of a working Orra project)

## A quick start guide to set up and run an Orra project locally

**Requirements**:
- [Poetry installed](https://python-poetry.org/docs/#installation).
- Clone this repository.

1. **Create a new Orra project**:

```bash
poetry new orra-app
cd orra-app
```

2. **Install the Orra SDK locally from the cloned repository**:

```bash
poetry add /path/to/repo/libs/orra
```

3. **Install the Orra CLI locally from the cloned repository**:

```bash
poetry add /path/to/repo/libs/cli
```

4. **Create the project's main file in the `orra-app` directory**:

```bash
touch main.py
```

5. **Orchestrate your Agents** in the `main.py` file (see [example](examples/basics/basics/hello_world.py)).

6. Run your Orra project using the Orra CLI:

```bash 
poetry run python -m orra_cli run
````

7. **Your Orra project is now running**!

```bash
orra-app % poetry run python -m orra_cli run
  ✔ Compiling Orra application workflow... Done!
  ✔ Prepared Orra application step endpoints...Done!
  ✔ Preparing Orra application workflow endpoint... Done!
  ✔ Starting Orra application... Done!

  Orra development server running!
  Your API is running at:     http://127.0.0.1:1430

INFO:     Started server process [33823]
INFO:     Waiting for application startup.
INFO:     Application startup complete.
INFO:     Orra running on http://127.0.0.1:1430 (Press CTRL+C to quit)
```

Your orchestrated inputs and agents are now accessible **via HTTP endpoints**. Ready for testing.

8. **Execute your workflow** by sending a POST request to the `/workflow` endpoint:

```bash
curl -X POST \
  -H "Content-Type: application/json" \
  -d '{"source": null, "researched": null}' \ 
  http://127.0.0.1:1430/workflow
```

Outputs:

```json
{
	"researched": "'hello world' is a common phrase used in programming to demonstrate the basic syntax of a programming language. It is believed to have originated from the book \"The C Programming Language\" by Brian Kernighan and Dennis Ritchie.",
	"source": "hello world"
}
```

9. **Execute individual steps** by sending a POST request to the `/workflow/step_name` endpoint (e.g. `/workflow/investigate`):

```bash
curl -X POST \
  -H "Content-Type: application/json" \
  -d '{"source": null, "researched": null}' \
  http://127.0.0.1:1430/workflow/investigate
```

Outputs:

```json
{
	"researched": null,
	"source": "hello world"
}
```

This is a great way to test orchestrated steps individually.

10. 🎉 **You're all set!** 🎉

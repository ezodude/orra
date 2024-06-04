# 🪡 orra

🦸 Instantly orchestrate multi-agent workflows as services with Orra.

Orra's **Python SDK** and **Local Development Environment** enable agent-based service workflows with deployments,
and [workflow enhancements](#workflow-enhancements-coming-soon). This makes multi-agent orchestrations work seamlessly in
production.

## Why Orra?

You've built your agents, but they need a few tweaks before they're ready for production. This is where Orra comes in.🚀

Orra helps deploy your agents as services. It takes care of all the heavy lifting, so you can just focus on building and
improving your agents. ⚡️⚡️

## Mix and match agents

Seamlessly integrate any agents in a services based workflow. You can wire purpose-built agents on the web,
e.g. [GPT Researcher](https://github.com/assafelovic/gpt-researcher)
with custom agents built
with [LangChain](https://python.langchain.com/v0.1/docs/modules/agents/), [CrewAI](https://github.com/joaomdmoura/crewAI),
and more.

## Workflow enhancements (coming soon)

Orra bakes in enhancements to enable reliable, repeatable execution of complex multi agent-based service workflows by:

- Offering pre-built data and API integrations as part of the SDK.
- Standardizing flow control between agent-based services.
- Enhancing tool prompting via integrated LLM fine-tuning.
- Evaluating agent-service outputs to ensure correctness and quality.
- Monitoring costs across LLMs and tools.
- Offering pre-built open-source agents to get you up and running fast.

## We're just getting started

We're still ironing out the details of our **Local Development Environment**.

You can try out the latest by installing a local version of Orra.

(Check out the [Dependabot example](examples/dependabot/README.md) for a demo of a real-world agent-based service
workflow)

### What does Orra look like?

It just takes a few lines of code to orchestrate a service-based workflow using Orra:

```python
from typing import Optional, Any
from orra import Orra

app = Orra(schema={ "source": Optional[str], "researched": Optional[str]})

@app.step
def investigate(state: dict) -> Any:
    return { **state, "source": "hello world"}

@app.step
def research_topic(state: dict) -> Any:
   result = {} # Call your agent here
   return { **state, "researched": result }

# **** That's it! You've orchestrated your first service-based workflow using Orra. ****
```

### Try Orra locally 

This is a basic Hello World example to get you familiar with Orra.

**Requirements**:

- [Poetry installed](https://python-poetry.org/docs/#installation).
- Clone this repository.

1. **Create a new Orra project**:

```shell
poetry new orra-app
cd orra-app
```

2. **Install the Orra SDK and CLI locally from the cloned repository**:

```shell
poetry add /path/to/repo/libs/orra
poetry add /path/to/repo/libs/cli
```

3. **Create a main file in the `orra-app` directory**, and copy in the content
   of [this example](examples/basics/basics/hello_world.py):

```shell
touch main.py
```

4. **Run your Orra project using the Orra CLI**:

```shell 
poetry run python -m orra_cli run
````

5. **Your Orra project is now running**, and you can access it via HTTP endpoints! 🚀

```shell
poetry run python -m orra_cli run
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

6. **Execute your workflow as a service** by sending a POST request to the `/workflow` endpoint:

```shell
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

7. **Execute individual steps** by sending a POST request to the `/workflow/step_name` endpoint (
   e.g. `/workflow/investigate`):

```shell
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

🎉 **You're all set!** 🎉


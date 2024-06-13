# 🪡 orra

🦸 Dev-friendly platform purpose built to help you create a backend for your Agentic app.

## Why Orra?

We want you to share your Agentic app as soon as you're done prototyping your Agents or Crew.

But ... building resilient agentic apps is no trivial task. Their backends need robust recovery - if there's an outage,
agents should seamlessly resume from their last working state. Data access must be carefully restricted for security.
And we can't have them fabricating responses - accuracy is paramount.

And, that's just the tip of the iceberg, there are even more concerns that will keep you from shipping. The good news is
that Orra understands these hurdles.

Orra is built on [LangGraph](https://langchain-ai.github.io/langgraph/). It provides the right tools for you to quickly
build out and configure a backend that just works. Our aim is to provide infrastructure, inbuilt integrations and
dashboards to keep your agentic app running just right.

## We're just getting started

We're still ironing out the details.

### What does an Orra backend look like?

It should take a few lines of code to set up a basic backend using Orra:

```python
from typing import Optional, Any
from orra import Orra

app = Orra(schema={"source": Optional[str], "researched": Optional[str]})


@app.step
def investigate(state: dict) -> Any: # source relevant data
    return {**state, "source": "hello world"}


@app.step
def research_topic(state: dict) -> Any: # wire in your imported agent
    result = {}  # Call your agent here
    return {**state, "researched": result}

# **** That's it! You now have a `/flow` API endpoint, plus an API endpoint for each step. ****
```

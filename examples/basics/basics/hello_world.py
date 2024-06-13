from typing import Optional, Any

from orra import Orra

# This is the Orra instance that will be used to orchestrate your Agents
# along with any data sources and sinks as part of your backend.

# Orchestration creates a flow made up of a series of steps. Steps are
# executed in the order they are defined.

# All steps share state. The schema defines the structure of the
# state object that will be passed between steps.
app = Orra(
    schema={
        "source": Optional[str],
        "researched": Optional[str]
    }
)


# Define a step by using the `@app.step` decorator to annotate the `investigate` function.
# The function updates the state by returning a new state object.
# A step POST API endpoint is automatically created for this step at path: `/flow/investigate`.
@app.step
def investigate(state: dict) -> Any:
    return {
        **state,
        "source": "hello world",
    }


# Define a step by using the `@app.step` decorator to annotate the `research_topic` function.
# The function updates the state by returning a new state object.
# A step POST API endpoint is automatically created for this step at path: `/flow/research_topic`.
@app.step
def research_topic(state: dict) -> Any:
    # This function simulates using an agent to research a topic.
    def research_topic_using_agent(topic: str) -> str:
        return f"'{topic}' is a common phrase used in programming to demonstrate the basic syntax of a programming " \
               "language. It is believed to have originated from the book \"The C Programming Language\" by " \
               "Brian Kernighan and Dennis Ritchie."

    result = research_topic_using_agent(state['source'])
    return {
        **state,
        "researched": result
    }

# **** That's it! You've created a basic backend using Orra. ****

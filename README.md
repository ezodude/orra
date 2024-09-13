# Orra

Orra is a language-agnostic LLM powered orchestration platform. It provides building blocks to build reliable and fast
multi-agent applications. Developers can stop re-inventing the wheel and focus on adding customer value.

LLM-powered orchestration enables real-time adjustment of workflows based on context, intermediate results, and changing
requirements, allowing for more intelligent and flexible multi-agent systems.

## Key Benefits

1. **Dynamic AI-Powered Adaptability**: Real-time workflow adjustments based on context, ensuring optimal performance in
   changing environments.

2. **Reliable Multi-Agent Orchestration**: Build robust applications with deterministic task completion and automatic
   outage handling.

3. **Accelerated Task Execution**: Fast parallel processing enables concurrent execution of services and Agents for
   rapid task completion.

4. **Language-Agnostic Integration**: Use our SDKs to reliably orchestrate any service or Agent, regardless of
   programming language.

5. **Comprehensive Result Management**: Track interim results across services/Agents and accurately combine them into a
   single final output.

## Why LLM-Powered Orchestration?

LLM-powered orchestration enables real-time adjustment of workflows based on context, intermediate results, and changing
requirements, allowing for more intelligent and flexible multi-agent systems. Here's how it enhances AI workflow
management:

1. **Intelligent Workflow Generation**: Automatically create and adapt orchestration plans by understanding the
   capabilities of registered services and agents.
    - Decomposes complex tasks without manual intervention
    - Allocates resources optimally based on service capabilities
    - Handles task dependencies efficiently

2. **Dynamic Execution and Adaptation**: Continuously adjust workflows during runtime, responding to evolving conditions
   and interim outputs.
    - Optimizes task sequences in real-time
    - Implements intelligent error handling and recovery
    - Reallocates resources flexibly based on performance and changing needs

## Getting started

**Orra is in Alpha**. The core component is the control plane which is run as a server. It is available for Self-hosting
in Single User mode. We do not recommend running it in production yet.

You need to
have [Git](https://git-scm.com/book/en/v2/Getting-Started-Installing-Git), [Docker](https://docs.docker.com/desktop/)
and [Docker Compose](https://docs.docker.com/compose/install/) installed before you start.

1. Clone the repo.
    ```shell
      git clone https://github.com/ezodude/orra
    ```
2. Navigate to the control plane's root folder and run the control plane.
    ```shell
      cd controlplane
      cp env-example .env 
      docker compose up -d
    ```
3. Download the relevant Orra CLI binary and add it your path.
    ```shell
      mv orractl /user/local/bin/.
    ```
4. Login with the CLI and follow the instructions
    ```shell
      orractl login
    ```

## Using the Orra CLI

Run commands to set up projects, inspect orchestrations and generate API keys using the Orra command-line tool.

```shell
orractl --help
# orractl manages Orra and orchestration workflows. 
# Usage:  orractl [OPTIONS] COMMAND
# projects    Add and manage projects
# webhooks    Add and manage webhooks for a project
# keys        Add and manage API_KEYS for a project
# ps          List orchestrations for a project
# inspect     Return information of an orchestration
# logs        Fetch the logs for an orchestration
# login       Log in to a registry
# logout      Log out from a registry
# version     Print the client and server version information
```

## Orchestration

A multi-agent application may consist of many components. Orra orchestrates the LLM based Agents and related services,
e.g. data ingestion services, that run the core of your application. It assumes both Agents and services are run as
microservices, always available to accept data. While an orchestration plan is executing, Orra checks whether the
necessary Agents and services are online. If a necessary service is offline, task execution is halted until it's back
online.

It is up to you how your Agents and services are deployed. But, we strongly recommend they are deployed and run as
containers for optimal performance. Checks are

To start orchestrating Agents and services for your application you have to log in and add a project.

```shell
orractl projects add new-orra-project
```

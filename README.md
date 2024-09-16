# Orra

Orra is a language-agnostic LLM powered orchestration platform. It provides building blocks to build reliable and fast
multi-agent applications. Developers can stop re-inventing the wheel and focus on adding customer value.

## Key Benefits

1. **Dynamic AI-Powered Adaptability**: Automatically adjust workflows in real-time based on context, ensuring optimal
   performance in changing environments.

2. **Build Reliable Multi-Agent Systems**: Create robust applications with deterministic task completion and automatic
   outage handling. **\[Coming Soon\]**

3. **Accelerate Task Execution**: Enable fast parallel processing for concurrent execution of services and Agents.

4. **Integrate Across Languages**: Utilize our SDKs to reliably orchestrate any service or Agent, regardless of
   programming language. **Starting with JavaScript/TypeScript**

5. **Manage Results Comprehensively**: Track interim results across services/Agents and accurately combine them into a
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
# api-keys    Add and manage API keys for a project
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
microservices, always available to accept data.

Orchestration plans are constantly adjusted to ensure tasks are completed using the available Agents and services. For
example, if a necessary service is offline, task execution maybe halted until it's back online.

It is up to you how your Agents and services are deployed. But, we strongly recommend they are deployed and run as
containers for optimal performance.

### Configure orchestration for your project

1. Log in and add a project.
   ```shell
      orractl projects add new-orra-project
   ```
2. Add a webhook to accept orchestration results.
   ```shell
      orrctl webhooks add --url "http://localhost:3000/webhooks/orra" -p new-orra-project
   ```
3. Generate an API key to authenticate and orchestrate tasks. The new API key is required for use in Orra SDKs.
   ```shell
      orrctl api-keys add --name 'My API Key' -p new-orra-project
   ```

### Setup Agents and services for orchestration

You can use a preferred language SDK to register your Agents and services with our control plane.

Here's an example using the JavaScript SDK.

1. Give your Agent or service a unique name.
   Ensure the name can be used as a DNS subdomain name as defined in RFC 1123. This means the name must:
   - contain no more than 253 characters
   - contain only lowercase alphanumeric characters, '-' or '.'
   - start with an alphanumeric character
   - end with an alphanumeric character

2. Give your Agent or service a concise description that clearly explain what it does.
   The description cannot be longer than 500 chars. 

3. Define the expected input and output schema.
   ```javascript
      const serviceSchema = {
        input: {
            type: 'object',
            fields: [ { name: 'customerId', type: 'string', format: 'uuid' } ],
            required: [ 'customerId' ]
        },
        output: {
            type: 'object',
            fields: [
                { name: 'id', type: 'string', format: 'uuid' },
                { name: 'name', type: 'string' },
                { name: 'balance', type: 'number', minimum: 0 }
            ]
        }
   };
   ```
4. Set up the task handler, this is a function called by the SDK that will kick off your Agent or service's work.
   - It will receive an input object that conforms to the agent/service input schema.
   - It will output data as an object that conforms to the agent/service output schema.

5. Add a version to the service, this useful for logging and general system debugging. 

Here's the full service setup for Orra orchestration.
```javascript
const { createClient, setHandler } = require('@orra/sdk');

// Create a client
const orraClient = createClient({
   orraUrl: process.env.ORRA_URL,
   orraKey: process.env.ORRA_API_KEY
});

// Flesh out your services inputs and outputs
const serviceSchema = {
     input: {
         type: 'object',
         fields: [ { name: 'customerId', type: 'string', format: 'uuid' } ],
         required: [ 'customerId' ]
     },
     output: {
         type: 'object',
         fields: [
             { name: 'id', type: 'string', format: 'uuid' },
             { name: 'name', type: 'string' },
             { name: 'balance', type: 'number', minimum: 0 }
         ]
     }
};

async function main() {
   try {
       // Register your service or Agent, clearly explain what it does.
       await orraClient.registerService(
            'CustomerAccountService',
            {
              description: 'Retrieves and manages customer account data' ,
              schema: serviceSchema,
              // Setup a handler function that performs the work for this service or Agent.
              taskHandler: handler,
              version: '1.0.0'
            }
       );
       
       console.log('Service registered successfully');
     } catch (error) {
       console.error('Registration failed:', error);
     }
}

// This will receive input as per the input schema setup previously
function handler(taskData) {
   console.log('Received task:', task);
   
   // Process the task
   // ..
   
   return { status: 'completed', result: 'Processed data' };
}

main();

process.on('SIGINT', () => { 
   console.log('Closing connection...');
   orraClient.close();
   process.exit();
});
```

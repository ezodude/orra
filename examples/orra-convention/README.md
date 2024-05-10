# Orra workflow convention

An example that illustrates the orra convention for setting up a workflow project.

To track state, when creating the project:
- Ask the user to provide state values and types
- These can be provided as flags to the `create` command, similar to how docker accepts env vars
- This will be used to generate an orra state python module.
- In the ```__main__``` file The user can then import the state module.
- The user can then use the state module to access the state values.
- The user can also use the state module to update the state values.
- The user can pass any state values to the Agent.
- Any values returned by the Agent can be used to update the state, in ```__main```.

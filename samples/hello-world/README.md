# Cadence Golang Hello World

This is a simple hello world example for Cadence workflow engine using the Go client.

## Prerequisites

Before running this sample, make sure you have:

1. **Cadence server running**: You can start it using Docker:
   ```bash
   docker-compose -f docker/docker-compose.yml up
   ```

2. **Domain registered**: Register the `test-domain` domain:
   ```bash
   cadence --domain test-domain domain register
   ```
   
   Or using the dockerized CLI:
   ```bash
   docker run --network=host --rm ubercadence/cli:master --domain test-domain domain register
   ```

## Building and Running the Worker

1. **Build the worker:**
   ```bash
   cd /Users/zawadzki/Uber/cadence/samples/hello-world
   GOWORK=off go build -o hello-world
   ```

2. **Run the worker:**
   ```bash
   ./hello-world
   ```

   You should see logs like:
   ```
   INFO  Started Worker.  {"worker": "test-worker"}
   ```

## Running the Workflow

In a separate terminal, while the worker is running, execute the workflow:

### Using Cadence CLI (if installed locally):
```bash
cadence --domain test-domain workflow start --et 60 --tl test-worker --workflow_type main.helloWorldWorkflow --input '"World"'
```

### Using Docker:
```bash
docker run --network=host --rm ubercadence/cli:master --domain test-domain workflow start --et 60 --tl test-worker --workflow_type main.helloWorldWorkflow --input '"World"'
```

## Expected Output

In your worker terminal, you should see logs similar to:

```
INFO  helloworld workflow started
INFO  helloworld activity started
INFO  Workflow completed.  {"Result": "Hello World!"}
```

## Monitoring with Cadence Web UI

Open your browser and navigate to:
```
http://localhost:8088
```

Enter `test-domain` in the domain field to view the workflow execution history.

## What's Happening

This example demonstrates:

1. **Activity (`helloWorldActivity`)**: A simple function that takes a name parameter and returns a greeting message
2. **Workflow (`helloWorldWorkflow`)**: Orchestrates the activity execution with timeout configurations
3. **Worker Service**: Registers the workflow and activity, polls for tasks, and executes them

The worker continuously listens for workflow and activity tasks on the `test-worker` task list and executes them when triggered via the CLI.


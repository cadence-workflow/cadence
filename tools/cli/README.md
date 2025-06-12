Documentation for the Cadence command line interface is located at our [main site](https://cadenceworkflow.io/docs/cli/).

### Build CLI binary locally

To build the CLI tool locally check out the version tag (e.g. `git checkout v0.21.3`) and run `make tools`. 
This produces an executable called cadence.

Run help command with a local build:
````
./cadence --help
````

Command to describe a domain would look like this:
````
./cadence --domain samples-domain domain describe
````

# Cadence CLI Natural Language Interface

## Overview

The Cadence CLI includes a natural language interface that allows you to ask questions in plain English and get back proper Cadence CLI commands.

## Usage

```bash
./cadence-cli ask "show me failed workflows"
./cadence-cli nl "list all domains"
```

## Setup Requirements

### 1. Install Claude

Install the Claude CLI tool from Anthropic. The CLI will automatically detect Claude in these locations:

- System PATH
- `/usr/local/bin/claude`
- `/opt/homebrew/bin/claude` (macOS Homebrew)
- `$HOME/.local/bin/claude`
- `$HOME/.nvm/versions/node/*/bin/claude` (Node.js/nvm installations)

Alternatively, set the `CLAUDE_PATH` environment variable to point to your Claude installation:

```bash
export CLAUDE_PATH="/path/to/your/claude"
```

### 2. Configure Vertex AI (if using Google Cloud)

Set up the required environment variables:

```bash
# Required: Your Google Cloud project ID
export ANTHROPIC_VERTEX_PROJECT_ID="your-project-id"

# Optional: Cloud region (defaults to us-east5)
export CLOUD_ML_REGION="us-east5"

# Required for Vertex AI
export CLAUDE_CODE_USE_VERTEX=1
```

### 3. Authenticate with Google Cloud

```bash
gcloud auth application-default login
gcloud auth application-default set-quota-project your-project-id
```

## Examples

```bash
# Workflow queries
./cadence-cli ask "show me failed workflows"
./cadence-cli ask "list running workflows"
./cadence-cli ask "find workflows that started today"

# Domain management
./cadence-cli ask "list all domains"
./cadence-cli ask "create a domain called test-domain"
./cadence-cli ask "describe domain my-domain"

# Administrative tasks
./cadence-cli ask "terminate workflow with id abc123"
```

## Troubleshooting

### "claude not found" error
- Ensure Claude is installed and accessible
- Set `CLAUDE_PATH` environment variable if needed
- Check that Claude is in your system PATH

### "ANTHROPIC_VERTEX_PROJECT_ID environment variable not set" error
- Set the required environment variable with your Google Cloud project ID
- Ensure you have proper authentication set up

### Authentication errors
- Run `gcloud auth application-default login`
- Verify your project has the necessary APIs enabled
- Check that your service account has proper permissions

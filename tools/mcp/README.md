## Overview

This folder contains an [MCP server](https://modelcontextprotocol.io/introduction) which exposes useful Cadence tools to Cursor.

## How to install
This should be integrated to Cursors inside devpod seamlessly. For now, follow manual steps below:

1. Build the server executable
```
mkdir -p .bin && go build -o .bin/cadence_mcp tools/mcp/main.go
```


2. Update .cursor/mcp.json with following entry. Use the full path to the executable:
```
{
"mcpServers": {
  "cadence-mcp-server": {
      "command": "/path/to/repo/.bin/cadence_mcp",
      "args": [],
      "env": {}
    }
  }
}
```

3. Enable Agent mode in Cursor.

4. Enable yolo mode if you want tools to be run without confirmation.

5. Restart Cursor

## Usage

Ask a relevant question. For example:

  Is my Cadence domain "cadence-system" resilient to regional outages?

For now, it will tell you "Yes" if the domain is global, and "No" otherwise.

## How to add a new tool

1. Implement the tool in main.go
2. Build the server executable
3. Restart Cursor
4. Ask a relevant questions and test it out

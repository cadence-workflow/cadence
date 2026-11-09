# Managing Dynamic Config via CLI

## Overview

Cadence's dynamic configuration system allows you to change server behavior without restarting services. Configuration values can be updated at runtime and applied immediately.

This guide shows how to safely manage dynamic config using the Cadence CLI's admin commands.

## Requirements

### Configstore-Based Dynamic Config

The server must be configured to use **configstore-based** dynamic config (not file-based). With configstore mode, config values are stored in the database and can be updated via CLI or API.

### Static Configuration

Add the following to your Cadence server's static configuration file (e.g., `config/development.yaml`):

```yaml
dynamicconfig:
  # Use configstore mode (values stored in database)
  client: configstore
```

**Configuration modes:**
- **`configstore`**: Config values stored in database, updatable via CLI/API
- **`filebased`**: Config loaded from YAML files, changes take effect automatically when files are updated

> **Note**: If your server uses file-based config, you cannot use the CLI commands shown in this guide. You must edit the YAML files directly (changes are picked up automatically by the polling mechanism).

> **Important**: Dynamic config values are **not replicated** across clusters. In a multi-cluster deployment (e.g., for global domains), you must update the config in each cluster separately.

## Understanding Dynamic Config Commands

The CLI provides these admin commands for managing dynamic config:

| Command | Purpose |
|---------|---------|
| `admin config list` | List all stored config values (or for a specific key) |
| `admin config get` | Get the effective value for a config key with optional filters |
| `admin config update` | Update config values (global default or with filters) |
| `admin config restore` | Remove config values (restore to defaults) |
| `admin config listall` | List all available config keys with descriptions and default values|

### Operational Config Commands

For operational dynamic config (cassandra-backed store), use the `operational-*` variants:
- `admin config operational-get`
- `admin config operational-update`
- `admin config operational-restore`
- `admin config operational-list`

## Practical Example: Rate Limiting a Domain

This example demonstrates how to manage the `frontend.globalDomainrps` config, which controls the rate limit (requests per second) for workflow APIs per domain across the entire Cadence cluster.

### Step 1: List Current Config Values

Check what values are currently stored for this config:

```bash
cadence admin config list --name frontend.globalDomainrps
```

**Example output (no values set):**
> **Note**: If this config key has never been set, the server currently returns ALL stored config entries (for all keys), not an empty result. Filter to the specific key with: `jq 'map(select(.Name == "frontend.globalDomainrps"))[0].Values'`

**Example output (values exist):**
```json
[
  {
    "Name": "frontend.globalDomainrps",
    "Values": [
      {
        "Value": 1000,
        "Filters": []
      },
      {
        "Value": 100,
        "Filters": [
          {
            "Name": "domainName",
            "Value": "rate-limited-domain"
          }
        ]
      }
    ]
  }
]
```

### Step 2: Get Effective Value for a Domain

Get the effective value that will be used for a specific domain:

```bash
# Get global default (no filter)
cadence admin config get --name frontend.globalDomainrps

# Get value for a specific domain
cadence admin config get \
  --name frontend.globalDomainrps \
  --filter '{"domainName": "my-domain"}'
```

**How filtering works:**
- If a value with matching filters exists, it's returned
- Otherwise, the global default (no filters) is returned
- If no values exist at all, the default value defined in code is returned

### Step 3: Update Config Values

You can update config in two ways: from a JSON file (recommended) or inline JSON.

#### Option A: From JSON File (Recommended)

For configurations with domain-specific overrides, use a file. The recommended workflow is to extract the current config, modify it, then apply it back:

**1. Extract current config to a file using jq:**
```bash
# Extract the Values array (filters to this key in case server returns all keys)
cadence admin config list --name frontend.globalDomainrps | \
  jq 'map(select(.Name == "frontend.globalDomainrps"))[0].Values' > frontend-rps-config.json
```

**2. Edit the file to add/modify/remove overrides:**

Edit `frontend-rps-config.json` to make your changes. The file format should be:
```json
[
  {
    "Value": 1000,
    "Filters": []
  },
  {
    "Value": 2000,
    "Filters": [
      {
        "Name": "domainName",
        "Value": "critical-domain"
      }
    ]
  },
  {
    "Value": 100,
    "Filters": [
      {
        "Name": "domainName",
        "Value": "rate-limited-domain"
      }
    ]
  },
  {
    "Value": 500,
    "Filters": [
      {
        "Name": "domainName",
        "Value": "staging-domain"
      }
    ]
  }
]
```

**3. Apply the configuration:**
```bash
cadence admin config update \
  --name frontend.globalDomainrps \
  --value-file frontend-rps-config.json
```

**Advantages of file-based updates:**
- Easier to review before applying (use `git diff`)
- Can be version-controlled
- Supports complex configurations with many overrides
- Reduces typos in large JSON objects

#### Option B: Inline JSON (Not Recommended)

> **Warning**: This option is not recommended and may be deprecated in the future. Use `--value-file` instead for better safety and maintainability.

For simple single-value updates, you can use inline JSON:

```bash
# Set global default to 1000 rps
cadence admin config update \
  --name frontend.globalDomainrps \
  --value '{"Value": 1000, "Filters": []}'

# Add domain-specific override for "critical-domain" (2000 rps)
cadence admin config update \
  --name frontend.globalDomainrps \
  --value '{"Value": 1000, "Filters": []}' \
  --value '{"Value": 2000, "Filters": [{"Name": "domainName", "Value": "critical-domain"}]}'

# Lower rate limit for "rate-limited-domain" (100 rps)
cadence admin config update \
  --name frontend.globalDomainrps \
  --value '{"Value": 1000, "Filters": []}' \
  --value '{"Value": 2000, "Filters": [{"Name": "domainName", "Value": "critical-domain"}]}' \
  --value '{"Value": 100, "Filters": [{"Name": "domainName", "Value": "rate-limited-domain"}]}'
```

> **Important**: The `update` command **replaces all values** for the config key. You must include all values you want to keep.

### Step 4: Verify the Update

After updating, verify the new values are applied:

```bash
# List all values for this config
cadence admin config list --name frontend.globalDomainrps

# Test effective value for a specific domain
cadence admin config get \
  --name frontend.globalDomainrps \
  --filter '{"domainName": "critical-domain"}'
```

Expected output for critical-domain:
```
2000
```

### Step 5: Restore to Defaults (if needed)

To remove custom config values and restore defaults:

#### Remove Global Default Value (restore to code default)

```bash
cadence admin config restore --name frontend.globalDomainrps
```

This removes the global default value. The config will use the code default.

#### Remove Domain-Specific Override Only

```bash
cadence admin config restore \
  --name frontend.globalDomainrps \
  --filter '{"domainName": "rate-limited-domain"}'
```

This removes only the value matching the filter. The global default and other domain-specific overrides are kept.

## Best Practices & Safety Tips

### 1. Always Review Before Updating

```bash
# List current state before making changes
cadence admin config list --name <config-key>

# For file-based updates, review the file
cat config.json | jq .

# After update, verify immediately
cadence admin config list --name <config-key>
```

### 2. Prefer File-Based Updates

Always use `--value-file` instead of inline `--value`:
- Version control your config files
- Review changes with `git diff`
- Test in staging environment first
- Keep a backup of previous working config
- Avoid typos in complex JSON structures
- Inline `--value` may be deprecated in the future

### 3. Update Command Replaces All Values

When you run `update`, it **replaces** the entire config (all values for that key). To add a new domain override, you must include:
- The existing global default
- All existing domain-specific overrides
- The new override you're adding

**Wrong approach (will delete other overrides):**
```bash
# DON'T DO THIS - will delete all other values!
cadence admin config update \
  --name frontend.globalDomainrps \
  --value '{"Value": 100, "Filters": [{"Name": "domainName", "Value": "new-domain"}]}'
```

**Correct approach:**
```bash
# Get current config values
cadence admin config list --name frontend.globalDomainrps | \
  jq 'map(select(.Name == "frontend.globalDomainrps"))[0].Values' > current.json

# Edit current.json to add new override
# Review changes: git diff current.json

# Apply with all values
cadence admin config update \
  --name frontend.globalDomainrps \
  --value-file current.json
```

### 4. Test Filter Matching

Before applying a filtered value, test that the filter will match correctly:

```bash
# After update, verify the filter works
cadence admin config get \
  --name frontend.globalDomainrps \
  --filter '{"domainName": "test-domain"}'
```

### 5. Avoid Empty Arrays in Files

Never save an empty JSON array to a config file - it will be rejected:

```bash
# DON'T create files like this
echo '[]' > config.json

# Error: file "config.json" contains no config values
# (empty array would delete all values for this config)
```

### 6. Monitor Config Changes

- Set up alerts for critical config changes
- Log all config updates with who/when/what
- Review config in post-mortems when investigating incidents

### 7. Gradual Rollout

For rate limit changes that might impact production:
1. Start with a higher limit
2. Monitor metrics (latency, error rates)
3. Gradually decrease if system handles it well
4. Keep the old config file as a quick rollback option

### 8. Multi-Cluster Deployments

Dynamic config is **not replicated** across clusters:
- Each cluster stores config in its own database
- Changes must be applied to each cluster separately
- For global domains spanning multiple clusters, update all clusters to maintain consistency
- Consider using version-controlled config files and automation to keep clusters in sync

## Common Use Cases

### Rate Limiting by Domain

Control request rates per domain to prevent noisy domains from affecting others:

```bash
# Extract current config
cadence admin config list --name frontend.globalDomainrps | \
  jq 'map(select(.Name == "frontend.globalDomainrps"))[0].Values' > rps-config.json

# Edit rps-config.json to add domain-specific limit
# Add: {"Value": 100, "Filters": [{"Name": "domainName", "Value": "noisy-domain"}]}

# Apply the updated config
cadence admin config update \
  --name frontend.globalDomainrps \
  --value-file rps-config.json
```

### Emergency Rate Limit Reduction

Quickly reduce load during an incident while preserving per-domain overrides:

```bash
# Extract current config
cadence admin config list --name frontend.globalDomainrps | \
  jq 'map(select(.Name == "frontend.globalDomainrps"))[0].Values' > emergency-backup.json

# Edit the global default (first entry with "Filters": [])
# Change "Value": 1000 to "Value": 500
# Keep all domain-specific entries unchanged

# Apply the updated config
cadence admin config update \
  --name frontend.globalDomainrps \
  --value-file emergency-backup.json

# After incident, restore from version control or backup
```

### Per-Domain Feature Flags

Many configs support domain-based filtering. Example with a boolean config:

```bash
# Extract current config
cadence admin config list --name system.enableFeatureX | \
  jq 'map(select(.Name == "system.enableFeatureX"))[0].Values' > feature-config.json

# Edit feature-config.json to enable for canary domain:
# [
#   {"Value": false, "Filters": []},
#   {"Value": true, "Filters": [{"Name": "domainName", "Value": "canary-domain"}]}
# ]

# Apply the config
cadence admin config update \
  --name system.enableFeatureX \
  --value-file feature-config.json
```

## Troubleshooting

### "invalid dynamic config key name"

The config key doesn't exist. List available keys:
```bash
cadence admin config listall
```

### Changes Not Taking Effect

- Check your server is using `configstore` mode, not `filebased`
- Verify the config update succeeded (check `list` output)
- Some configs may have a poll interval (default is seconds)
- Check server logs for config update events

### "mutually exclusive" Error

You cannot use both `--value` and `--value-file` in the same command:
```bash
# ERROR
cadence admin config update --name X --value '{}' --value-file config.json

# CORRECT: Choose one
cadence admin config update --name X --value '{}'
# OR
cadence admin config update --name X --value-file config.json
```

### Accidentally Deleted All Values

Restore the config from your backup file:
```bash
cadence admin config update \
  --name frontend.globalDomainrps \
  --value-file backup-config.json
```

If you don't have a backup, the config will use the code default until you set new values.

## Reference

### Config Value JSON Format

```json
{
  "Value": <any-json-value>,
  "Filters": [
    {
      "Name": "<filter-name>",
      "Value": <filter-value>
    }
  ]
}
```

**Supported Value Types:**
- Numbers: `"Value": 1000`
- Booleans: `"Value": true`
- Strings: `"Value": "enabled"`
- Objects: `"Value": {"key": "value"}`
- Arrays: `"Value": [1, 2, 3]`

**Common Filter Names:**
- `domainName`: Filter by workflow domain
- `taskListName`: Filter by task list
- `workflowType`: Filter by workflow type
- `clusterName`: Filter by cluster name

### File Format for --value-file

Must be a JSON array of value objects:

```json
[
  {
    "Value": <value1>,
    "Filters": [...]
  },
  {
    "Value": <value2>,
    "Filters": [...]
  }
]
```

## See Also

- [CLI Reference](../../tools/cli/README.md) - Full CLI documentation
- Available config keys: Run `cadence admin config listall`
- [Dynamic Config Source Code](../../common/dynamicconfig/dynamicproperties/constants.go) - All config key definitions

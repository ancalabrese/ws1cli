# ws1cli

A command-line interface for [Workspace ONE UEM](https://www.omnissa.com/workspace-one/) (Omnissa).

Commands are generated directly from the official WS1 UEM OpenAPI spec, so the CLI stays in sync with the API surface.

## Installation

```bash
go install github.com/ancalabrese/ws1cli/cmd/ws1@latest
```

Or build from source:

```bash
make build        # outputs bin/ws1
make install      # installs to $GOPATH/bin
```

## Setup

**1. Set your server:**

```bash
ws1 config --server as1831.awmdm.com --version 2506
```

**2. Authenticate with OAuth2 client credentials:**

```bash
ws1 login --client-id <client-id> --secret <client-secret>
```

By default the auth region is `na`. Use `--region` to pick another (`na`, `emea`, `apac`, `uat`).

Tokens are refreshed automatically before expiry.

**3. Verify config:**

```bash
ws1 config
```

Configuration is stored at `~/.config/ws1/config.json`.

## Usage

```
ws1 <group> <subgroup> <operation> [flags]
```

Commands are organised by API group:

| Group | Description |
|-------|-------------|
| `mam` | Mobile Application Management |
| `mcm` | Mobile Content Management |
| `mdm` | Mobile Device Management |

All commands output pretty-printed JSON.

### Example

```bash
ws1 mam apps list
ws1 mdm devices search --serial-number ABC123
```

### Global flags

| Flag | Env var | Description |
|------|---------|-------------|
| `--server` | `WS1_SERVER` | Server hostname override |
| `--token` | `WS1_TOKEN` | Bearer token override (disables auto-refresh) |
| `--tenant` | `WS1_TENANT` | `aw-tenant-code` header override |

## Development

**Regenerate everything from spec:**

```bash
make gen          # gen-client + gen-cli
make gen-client   # API client (ws1/client.gen.go) via oapi-codegen
make gen-cli      # Cobra commands (internal/cli/gen/) via cmd/gen_cli
```

**Validate the spec:**

```bash
make validate
```


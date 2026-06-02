# Generic LLM Provider Setup

The CLI and control plane use a provider-neutral chat-completions contract.
Do not bind architecture code or documentation to a specific model, vendor, or
hosted service. Concrete endpoints stay in local environment variables or a
future secret backend.

## Environment Variables

```powershell
$env:LLM_BASE_URL="<chat-completions-compatible-base-url>"
$env:LLM_MODEL="<chat-model-id>"
$env:LLM_API_KEY="<optional-bearer-token>"

$env:VLM_BASE_URL="<vision-compatible-base-url>"
$env:VLM_MODEL="<vision-model-id>"
$env:VLM_API_KEY="<optional-bearer-token>"
```

Local development may point these variables at any compatible endpoint. The
repository should only store the generic contract, not real keys or provider
specific defaults.

## Mimo Local Test Provider

For local interactive testing, Mimo can be used through compatible endpoints.
Do not commit the real key. Do not place it in browser code. If the subscription
terms limit usage to interactive coding or agent tools, do not use that key for
public services, automated backend jobs, or unattended production workflows.

Current `labelctl` planning uses an OpenAI-compatible chat-completions contract:

```powershell
$env:LLM_BASE_URL="https://token-plan-cn.xiaomimimo.com/v1"
$env:LLM_MODEL="mimo-v2.5-pro"
$env:LLM_API_KEY="<local-secret>"

$env:VLM_BASE_URL="https://token-plan-cn.xiaomimimo.com/v1"
$env:VLM_MODEL="mimo-v2.5"
$env:VLM_API_KEY="<local-secret>"
```

For Anthropic-compatible tools such as Claude Code, keep the settings local:

```powershell
$env:ANTHROPIC_BASE_URL="https://token-plan-cn.xiaomimimo.com/anthropic"
$env:ANTHROPIC_AUTH_TOKEN="<local-secret>"
$env:ANTHROPIC_MODEL="mimo-v2.5-pro"
$env:ANTHROPIC_DEFAULT_SONNET_MODEL="mimo-v2.5-pro"
$env:ANTHROPIC_DEFAULT_OPUS_MODEL="mimo-v2.5-pro"
$env:ANTHROPIC_DEFAULT_HAIKU_MODEL="mimo-v2.5-pro"
```

Model routing policy:

- `mimo-v2.5-pro`: planning, JSON plan generation, workflow reasoning.
- `mimo-v2.5`: vision checks for uploaded images or visual data.
- Any provider can replace Mimo if it declares the same capabilities.

## CLI

```powershell
$go = "$env:LOCALAPPDATA\Programs\Go\bin\go.exe"
& $go run .\cmd\labelctl llm ask "list available agent workflows"

$go = "$env:LOCALAPPDATA\Programs\Go\bin\go.exe"
& $go run .\cmd\labelctl llm agent
```

## Provider Contract

Every provider adapter should declare capability metadata before it is used by
a workflow:

- `supports_tools`
- `supports_vision`
- `supports_json_schema`
- `max_context_tokens`
- `streaming_support`
- `latency_class`
- `cost_class`
- `data_policy`
- `safety_profile`

Routing must use declared capabilities and policy requirements, not a hardcoded
provider name.

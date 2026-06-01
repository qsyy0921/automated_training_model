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

## CLI

```powershell
F:\keyan\token_compression\third_party\go1.26.3\go\bin\go.exe run .\cmd\labelctl llm ask "list available agent workflows"

F:\keyan\token_compression\third_party\go1.26.3\go\bin\go.exe run .\cmd\labelctl llm agent
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

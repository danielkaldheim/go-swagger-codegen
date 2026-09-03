# swagger-codegen

A lightweight Go CLI tool that generates Dart and asynchronous Python client SDKs from Swagger 2.0 specifications. The Python client uses `aiohttp` and can reuse Home Assistant's managed HTTP session.

## Build

```bash
go build -o swagger-codegen .
```

## Usage

```bash
swagger-codegen generate -i <swagger.json> [-o <output-dir>] [-c <config.json>]
```

### Flags

| Flag | Short | Required | Default | Description |
|------|-------|----------|---------|-------------|
| `--input` | `-i` | Yes | | Path to the Swagger 2.0 JSON spec |
| `--output` | `-o` | No | `output` | Output directory for generated code |
| `--config` | `-c` | No | | Path to a config JSON file |
| `--language` | `-l` | No | `dart` | Target language: `dart` or `python` |
| `--variant` | | No | | Language variant (e.g. `blocks` for `dart-blocks` templates) |
| `--prune-unused-models` | | No | `false` | Only generate models reachable from the (non-excluded) API surface. Overrides `pruneUnusedModels` in the config when set. |

### Examples

Generate with defaults:

```bash
swagger-codegen generate -i swagger.json
```

Generate with a config file and custom output directory:

```bash
swagger-codegen generate -i swagger.json -c config.json -o lib/generated
```

Generate a Python package:

```bash
swagger-codegen generate -i swagger.json -l python -c config.json -o famn-sdk
```

## Config File

The optional config file is a JSON object with the following fields:

```json
{
  "pubName": "my_sdk",
  "pubVersion": "1.0.0",
  "pubDescription": "A Dart SDK for my API",
  "packageName": "my_python_sdk",
  "packageVersion": "1.0.0",
  "packageDescription": "An async Python SDK for my API",
  "pythonRequires": ">=3.11",
  "browserClient": false,
  "useEnumExtension": true,
  "excludeApi": ["EntryApi"],
  "excludeModel": ["AppleReferralData"],
  "pruneUnusedModels": false,
  "keepModel": ["OrderFilter"]
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `pubName` | string | `swagger` | Dart package name (used in `pubspec.yaml` and `part of` directives) |
| `pubVersion` | string | `1.0.0` | Package version in `pubspec.yaml` |
| `pubDescription` | string | `Swagger API client` | Package description in `pubspec.yaml` |
| `packageName` | string | value of `pubName` | Python import package name; normalized to snake case |
| `packageVersion` | string | value of `pubVersion` | Python project version in `pyproject.toml` |
| `packageDescription` | string | value of `pubDescription` | Python project description |
| `pythonRequires` | string | `>=3.11` | Supported Python version constraint |
| `browserClient` | bool | `false` | Reserved for browser client support |
| `useEnumExtension` | bool | `true` | Use `x-enum-values` vendor extension for enum generation |
| `excludeApi` | string[] | `[]` | API class names (e.g. `EntryApi`) to skip generating |
| `excludeModel` | string[] | `[]` | Model class names to skip generating, regardless of usage |
| `pruneUnusedModels` | bool | `false` | Drop models not transitively reachable from the API surface. Applied *after* `excludeApi`/`excludeModel`, so excluding an API also prunes the models only it needed |
| `keepModel` | string[] | `[]` | Model class names to retain when `pruneUnusedModels` is on, even if no API references them (e.g. filter/query models). Their transitive dependencies are kept too. Unknown names are ignored |

## Dart Output Structure

```text
<output>/
  pubspec.yaml
  .analysis_options
  .gitignore
  README.md
  lib/
    api.dart                # Main library with part directives
    api_client.dart         # HTTP client with serialization
    api_exception.dart      # Exception class
    api_helper.dart         # Parameter helpers
    api_provider.dart       # Riverpod providers
    model/
      base_model.dart       # BaseModel abstract class
      <model>.dart          # One file per schema definition
    api/
      <tag>_api.dart        # One file per API tag
    auth/
      authentication.dart   # Auth interface
      http_basic_auth.dart
      api_key_auth.dart
      oauth.dart
```

## Python Output Structure

```text
<output>/
  pyproject.toml
  README.md
  src/
    <package_name>/
      __init__.py
      api_client.py         # Async aiohttp transport and authentication
      apis.py               # Endpoint classes grouped by Swagger tag
      exceptions.py
      models.py             # Dataclasses, enums, and type aliases
      py.typed
```

For Home Assistant, pass its shared session into the generated client:

```python
from homeassistant.helpers.aiohttp_client import async_get_clientsession
from famn_sdk import ApiClient

session = async_get_clientsession(hass)
client = ApiClient(session=session)
```

The generated client does not close an injected session. If it creates its own session, use it as an async context manager.

## Supported Swagger Features

- Schema definitions with properties, `$ref`, arrays, maps, enums, and type aliases
- Operations grouped by tag with path/query/header/body/form parameters
- Authentication methods (API key, HTTP basic, OAuth)
- Vendor extensions (`x-enum-values` for rich enum definitions)
- Multipart file upload form parameters

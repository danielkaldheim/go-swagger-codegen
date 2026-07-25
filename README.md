# swagger-codegen

A lightweight Go CLI tool that generates Dart client SDKs from Swagger 2.0 specifications. Drop-in replacement for the Dart generation of the Java-based [swagger-codegen](https://github.com/swagger-api/swagger-codegen), without the JVM dependency.

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
| `--language` | `-l` | No | `dart` | Target language for code generation |
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

## Config File

The optional config file is a JSON object with the following fields:

```json
{
  "pubName": "my_sdk",
  "pubVersion": "1.0.0",
  "pubDescription": "A Dart SDK for my API",
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
| `browserClient` | bool | `false` | Reserved for browser client support |
| `useEnumExtension` | bool | `true` | Use `x-enum-values` vendor extension for enum generation |
| `excludeApi` | string[] | `[]` | API class names (e.g. `EntryApi`) to skip generating |
| `excludeModel` | string[] | `[]` | Model class names to skip generating, regardless of usage |
| `pruneUnusedModels` | bool | `false` | Drop models not transitively reachable from the API surface. Applied *after* `excludeApi`/`excludeModel`, so excluding an API also prunes the models only it needed |
| `keepModel` | string[] | `[]` | Model class names to retain when `pruneUnusedModels` is on, even if no API references them (e.g. filter/query models). Their transitive dependencies are kept too. Unknown names are ignored |

## Output Structure

```
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

## Supported Swagger Features

- Schema definitions with properties, `$ref`, arrays, maps, enums, and type aliases
- Operations grouped by tag with path/query/header/body/form parameters
- Authentication methods (API key, HTTP basic, OAuth)
- Vendor extensions (`x-enum-values` for rich enum definitions)
- Multipart file upload form parameters

# Flow Diagrams

This directory contains auto-generated flow diagrams from the codebase.

## Auto-Generated Diagrams

### API Handler Call Flow

![API Handler Flow](api-handlers.svg)

This diagram shows the complete call graph of API endpoint handlers and their dependencies. Generated using `go-callvis`.

### CLI Command Execution Flow

![CLI Flow](cli-flow.svg)

Visualizes the CLI command structure and execution paths through the cobra command framework. Generated using `go-callvis`.

### Package Structure

See `structure.puml` for the PlantUML source of the package architecture.

## Files

- **api-handlers.svg** - API handler call flow
- **api-handlers.gv** - GraphViz source
- **cli-flow.svg** - CLI command execution flow
- **cli-flow.gv** - GraphViz source
- **structure.puml** - Package structure diagram

## Generating Locally

```bash
# Run the generation script
./tools/generate-diagrams.sh

# View diagrams
open docs/diagrams/api-handlers.svg
```

## Tools Used

- **go-callvis** - Generates call graphs from Go code
- **goplantuml** - Creates PlantUML diagrams from Go structures
- **graphviz** - Renders DOT files to SVG
- **plantuml** - Renders PlantUML to SVG

## GitHub Actions

Diagrams are automatically regenerated on every push to main when Go files change.
See `.github/workflows/generate-diagrams.yml`

## Manual Diagram Sources

Hand-crafted Mermaid diagrams are in the main README.md showing:
- Complete license flow (init → verify → activate)
- License check flow
- Proxy mode flow
- Direct mode flow

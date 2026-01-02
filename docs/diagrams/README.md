# Flow Diagrams

This directory contains auto-generated flow diagrams from the codebase.

## Auto-Generated Diagrams

These diagrams are automatically generated from the Go code:

- **api-handlers.svg** - API handler call flow
- **cli-flow.svg** - CLI command execution flow  
- **structure.puml/svg** - Package structure diagram

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

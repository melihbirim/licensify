#!/bin/bash
# Generate flow diagrams from Go code

set -e

echo "📊 Generating code flow diagrams..."

# Add Go bin to PATH
export PATH="$HOME/go/bin:$PATH"

# Install go-callvis if not present
if ! command -v go-callvis &> /dev/null; then
    echo "Installing go-callvis..."
    go install github.com/ofabry/go-callvis@latest
fi

# Create docs directory
mkdir -p docs/diagrams

# Generate call graph for main handlers
echo "Generating API handler flow..."
timeout 30s go-callvis -format=svg -file=docs/diagrams/api-handlers \
    -focus=github.com/melihbirim/licensify \
    -group=pkg,type \
    -nostd \
    -skipbrowser \
    . 2>/dev/null || echo "⚠️  Call graph generation timed out (this is normal for complex projects)"

# Generate for CLI
if [ -d "cmd/licensify-cli" ]; then
    echo "Generating CLI flow..."
    timeout 30s go-callvis -format=svg -file=docs/diagrams/cli-flow \
        -focus=github.com/melihbirim/licensify/cmd/licensify-cli \
        -group=pkg,type \
        -nostd \
        -skipbrowser \
        ./cmd/licensify-cli 2>/dev/null || echo "⚠️  CLI flow generation timed out"
fi

# Generate structure diagram using goplantuml
if ! command -v goplantuml &> /dev/null; then
    echo "Installing goplantuml..."
    go install github.com/jfeliu007/goplantuml/cmd/goplantuml@latest
fi

echo "Generating package structure..."
goplantuml -recursive -hide-fields -hide-methods ./internal > docs/diagrams/structure.puml 2>/dev/null || echo "⚠️  Package structure generation failed"

# Convert PlantUML to SVG if plantuml is available
if command -v plantuml &> /dev/null; then
    plantuml -tsvg docs/diagrams/structure.puml
else
    echo "💡 Install plantuml to convert .puml to .svg: brew install plantuml"
fi

echo ""
echo "✅ Diagrams generated in docs/diagrams/"
echo ""
echo "Generated files:"
ls -lh docs/diagrams/ 2>/dev/null || echo "No files generated yet"

echo ""
echo "💡 Tip: Add these to your README:"
echo "   ![API Flow](docs/diagrams/api-handlers.svg)"
echo "   ![CLI Flow](docs/diagrams/cli-flow.svg)"

# Flow Diagrams

This directory contains auto-generated flow diagrams from the codebase.

## Architecture Diagrams

### Deployment Architecture

```mermaid
graph TB
    subgraph "Production Deployment"
        LB[Load Balancer<br/>nginx/Caddy]
        App1[Licensify Instance 1<br/>:8080]
        App2[Licensify Instance 2<br/>:8080]
        DB[(PostgreSQL<br/>Primary)]
        DBReplica[(PostgreSQL<br/>Read Replica)]
        Redis[(Redis<br/>Rate Limit Cache)]
        Email[Email Service<br/>Resend]
    end
    
    subgraph "Development"
        DevApp[Licensify Dev<br/>:8080]
        SQLite[(SQLite<br/>activations.db)]
    end
    
    Client[Client Apps] --> LB
    LB --> App1
    LB --> App2
    
    App1 --> DB
    App2 --> DB
    App1 --> DBReplica
    App2 --> DBReplica
    
    App1 --> Redis
    App2 --> Redis
    
    App1 --> Email
    App2 --> Email
    
    DevClient[Dev Client] --> DevApp
    DevApp --> SQLite
    
    style LB fill:#4a90e2
    style App1 fill:#50c878
    style App2 fill:#50c878
    style DB fill:#ff6b6b
    style Redis fill:#ffa500
```

**Production Setup:**
- Load balancer distributes traffic across instances
- PostgreSQL for production reliability
- Redis for distributed rate limiting
- Read replicas for /check endpoint scaling

**Development Setup:**
- Single instance with SQLite
- No external dependencies required
- Perfect for testing and local development

### Rate Limiting Algorithm

```mermaid
flowchart TD
    Start[API Request] --> Auth{Valid License?}
    Auth -->|No| Unauth[401 Unauthorized]
    Auth -->|Yes| Hardware{Hardware ID Match?}
    Hardware -->|No| HWError[403 Hardware Mismatch]
    Hardware -->|Yes| Expired{License Expired?}
    Expired -->|Yes| ExpError[403 License Expired]
    Expired -->|No| DailyCheck{Daily Limit<br/>Exceeded?}
    
    DailyCheck -->|Yes| DailyLimit[429 Daily Limit<br/>Reset: Tomorrow]
    DailyCheck -->|No| MonthlyCheck{Monthly Limit<br/>Exceeded?}
    
    MonthlyCheck -->|Yes| MonthlyLimit[429 Monthly Limit<br/>Reset: Next Month]
    MonthlyCheck -->|No| IPCheck{IP Rate Limit<br/>100/min?}
    
    IPCheck -->|Exceeded| IPLimit[429 IP Rate Limited<br/>Try again in 1 min]
    IPCheck -->|OK| Increment[Increment Counters]
    
    Increment --> Cache[Update Redis/DB]
    Cache --> Process[Process Request]
    Process --> Success[200 Success]
    
    style Unauth fill:#ff6b6b
    style HWError fill:#ff6b6b
    style ExpError fill:#ff6b6b
    style DailyLimit fill:#ffa500
    style MonthlyLimit fill:#ffa500
    style IPLimit fill:#ffa500
    style Success fill:#50c878
```

**Rate Limit Hierarchy:**
1. **License Validation** - Auth + hardware binding
2. **Daily Quota** - Per-license daily limit
3. **Monthly Quota** - Per-license monthly limit  
4. **IP Rate Limit** - 100 requests/min per IP (DDoS protection)

**Counters Reset:**
- Daily: Midnight UTC
- Monthly: 1st of month UTC
- IP: Rolling 1-minute window

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

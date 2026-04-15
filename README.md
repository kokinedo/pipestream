# pipestream

Real-time data pipeline with AI classification.

A Go service that ingests live GitHub public events, classifies them using Claude AI, and presents results in a beautiful terminal dashboard. Events are persisted to SQLite and available via REST and WebSocket API.

## Features

- Real-time GitHub event ingestion with deduplication
- AI-powered classification using Claude: notable release, interesting project, security concern, trending, or routine
- Interestingness scoring for prioritized event surfacing
- Full terminal dashboard built with Bubbletea and Lipgloss
- SQLite persistence for durable event storage
- REST API and WebSocket endpoint for live updates
- Dry-run mode for testing without API calls

## Tech Stack

- Go 1.22+
- [Bubbletea](https://github.com/charmbracelet/bubbletea) -- terminal UI framework
- [Lipgloss](https://github.com/charmbracelet/lipgloss) -- terminal styling
- [Cobra](https://github.com/spf13/cobra) -- CLI framework
- SQLite via pure-Go driver
- Claude API for event classification

## Installation

```bash
git clone https://github.com/kokinedo/pipestream.git
cd pipestream
go build -o pipestream .
```

## Configuration

Set the following environment variables before running:

```bash
export ANTHROPIC_API_KEY="your-api-key"
```

## Usage

Start the pipeline with the TUI dashboard:

```bash
./pipestream
```

Run in dry-run mode (skips AI classification, uses mock data):

```bash
./pipestream --dry-run
```

Run in headless mode (no TUI, API only):

```bash
./pipestream --headless
```

## Architecture

```
GitHub Public Events
        |
    Ingester  -- polls events, deduplicates
        |
   Classifier -- Claude AI scoring and categorization
        |
      Store   -- SQLite persistence
       / \
      /   \
   Server  TUI
 (REST/WS) (Bubbletea dashboard)
```

## License

MIT

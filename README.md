# agextract

**Proof of Builder — Turn AI coding sessions into developer portfolios**

AI tools write a lot of code now. But who *steered* the AI? agextract parses transcripts from Claude Code, Cursor, Windsurf, and GitHub Copilot into structured timelines — then publishes public profiles that prove what you actually built and how you guided the process.

## What It Does

- **Upload transcripts** from Claude Code, Cursor, Windsurf, GitHub Copilot, or paste raw JSON
- **Auto-parse** conversations into structured timelines — user prompts, AI responses, tool calls, and code diffs
- **Public profiles** at `/@username/` with GitHub-style activity heatmaps, role distribution charts, and steering ratio
- **Steering tags** to annotate key human decisions: pivots, corrections, and architectural choices (with before/after snapshots)
- **CLI tool** (`agextract watch`) for automatic session syncing via filesystem watcher
- **REST API** with OAuth 2.0 bearer tokens for programmatic access

## Architecture

```
┌──────────────┐         ┌─────────────────────┐         ┌──────────────────────────┐
│   CLI (Go)   │──push──▶│  API (Django REST)   │◀───────▶│  Web App (Django + TW)   │
│              │         │  /api/v1/            │         │                          │
│ • watch      │         │  • OAuth 2.0 flow    │         │  • Upload & parse        │
│ • login      │         │  • Sessions CRUD     │         │  • Dashboard             │
│ • push       │         │  • Token auth        │         │  • Public profiles       │
│ • status     │         └─────────────────────┘         │  • Steering tags         │
└──────────────┘                    │                     │  • Charts (Chart.js)     │
       ↑                            │                     └──────────────────────────┘
  File watcher                   SQLite
  (fsnotify)
```

## Tech Stack

| Layer    | Technology                                     |
|----------|-------------------------------------------------|
| Backend  | Django 6.0, SQLite, Python 3                    |
| Frontend | Django templates, Tailwind CSS (CDN), HTMX, Chart.js |
| CLI      | Go 1.22, Cobra, fsnotify, BoltDB                |
| Auth     | OAuth 2.0 with bearer tokens (`agx_` prefix)   |

## Quick Start

### Web App

```bash
git clone <repo-url> && cd agextract
python -m venv venv && source venv/bin/activate
pip install -r requirements.txt
python manage.py migrate
python manage.py createsuperuser
python manage.py runserver
```

Visit `http://localhost:8000`, upload a transcript, and explore the parsed timeline.

### CLI

```bash
cd agextract-cli
go build -o agextract .
./agextract login          # authenticate via OAuth
./agextract watch           # watch for new sessions and auto-sync
./agextract push <file>     # push a single transcript
./agextract status          # check sync status
```

## Project Structure

```
agextract/
├── core/                  # Django app — models, parsers, web views
│   ├── models.py          # Session, Step, SteeringTag
│   ├── views.py           # Upload, dashboard, public profiles
│   └── urls.py
├── api/                   # REST API — OAuth, sessions CRUD
│   ├── models.py          # APIToken, OAuthCode
│   ├── views.py           # Token endpoints, session create/detail
│   └── urls.py
├── agextract-cli/         # Go CLI
│   ├── cmd/               # Cobra commands (watch, login, push, status)
│   └── internal/          # api, auth, config, queue, sources, watcher
├── templates/             # Django templates (Tailwind + HTMX)
│   ├── base.html
│   └── core/              # upload, dashboard, public_profile, session_detail
├── agextract/             # Django project settings & root URL conf
├── manage.py
└── requirements.txt
```

## API Endpoints

All API routes are under `/api/v1/`.

| Method | Endpoint                      | Description              |
|--------|-------------------------------|--------------------------|
| GET    | `/api/v1/oauth/authorize/`    | Start OAuth flow         |
| POST   | `/api/v1/oauth/token/`        | Exchange code for token  |
| POST   | `/api/v1/oauth/revoke/`       | Revoke a token           |
| GET    | `/api/v1/me/`                 | Current user info        |
| POST   | `/api/v1/sessions/`           | Create a session (JSON)  |
| POST   | `/api/v1/sessions/upload/`    | Upload a transcript file |
| GET    | `/api/v1/sessions/<id>/`      | Get session detail       |

## License

MIT

# clarr

> Automated cleanup manager for the *arr stack

clarr bridges the gap between Jellyfin, Radarr, Sonarr and your download client.
It listens to Jellyfin webhook events and automatically removes orphaned files,
unmonitors deleted media, and keeps your download folder clean — so you never
run out of disk space again.

---

## How it works

```
Jellyfin (delete event)
        │
        ▼
   clarr webhook
        │
        ├──▶ Radarr API  →  rescan + unmonitor missing movies
        ├──▶ Sonarr API  →  rescan + unmonitor empty series
        └──▶ Cleaner     →  delete orphaned files (hardlink count == 1)
```

---

## Features

- 🎣 **Jellyfin webhook receiver** — reacts in real time to media deletions
- 🔍 **Orphan detection** — finds files with no hardlink in your media folders
- 🗑️ **Automatic cleanup** — removes orphaned downloads and empty directories
- 📅 **Cron scheduler** — runs cleanup on a configurable schedule
- 🔄 **Radarr & Sonarr sync** — unmonitors deleted media automatically
- 🔒 **HMAC signature verification** — secures your webhook endpoint
- 🐳 **Docker ready** — single binary, scratch-based image
- 🌱 **Dry-run mode** — simulate cleanup without deleting anything

---

## Quick start

### Docker Compose

```yaml
services:
  clarr:
    image: ghcr.io/cleeryy/clarr:latest
    container_name: clarr
    restart: unless-stopped
    ports:
      - "8090:8090"
    environment:
      - CLARR_JELLYFIN_WEBHOOK_SECRET=changeme
      - CLARR_RADARR_URL=http://radarr:7878
      - CLARR_RADARR_API_KEY=your_api_key
      - CLARR_SONARR_URL=http://sonarr:8989
      - CLARR_SONARR_API_KEY=your_api_key
      - CLARR_QBITTORRENT_URL=http://qbittorrent:8080
      - CLARR_QBITTORRENT_USERNAME=admin
      - CLARR_QBITTORRENT_PASSWORD=changeme
      - CLARR_CLEANER_DOWNLOAD_DIR=/content/downloads
      - CLARR_CLEANER_DRY_RUN=true
      - CLARR_CLEANER_SCHEDULE=0 3 * * *
    volumes:
      - /content/downloads:/content/downloads:rw
    networks:
      - arr-network

networks:
  arr-network:
    external: true
```

### Configuration

Copy `.env.example` to `.env` and fill in your values.
You can also use `config.yaml` — environment variables always take priority.

| Variable | Description | Default |
|---|---|---|
| `CLARR_SERVER_PORT` | HTTP server port | `8090` |
| `CLARR_JELLYFIN_WEBHOOK_SECRET` | HMAC secret for webhook | **required** |
| `CLARR_RADARR_URL` | Radarr base URL | **required** |
| `CLARR_RADARR_API_KEY` | Radarr API key | **required** |
| `CLARR_SONARR_URL` | Sonarr base URL | **required** |
| `CLARR_SONARR_API_KEY` | Sonarr API key | **required** |
| `CLARR_QBITTORRENT_URL` | qBittorrent base URL | **required** |
| `CLARR_QBITTORRENT_USERNAME` | qBittorrent username | `admin` |
| `CLARR_QBITTORRENT_PASSWORD` | qBittorrent password | **required** |
| `CLARR_CLEANER_DOWNLOAD_DIR` | Path to downloads folder | **required** |
| `CLARR_CLEANER_DRY_RUN` | Simulate without deleting | `true` |
| `CLARR_CLEANER_SCHEDULE` | Cron expression for auto-cleanup | `0 3 * * *` |

---

## Jellyfin setup

1. Install the **Webhook plugin** in Jellyfin
2. Add a new webhook pointing to `http://clarr:8090/webhook/jellyfin`
3. Set the same secret as `CLARR_JELLYFIN_WEBHOOK_SECRET`
4. Enable the **Item Deleted** event

---

## API

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/health` | Health check |
| `GET` | `/api/stats` | Orphan files count and size |
| `POST` | `/api/cleanup` | Trigger manual cleanup |
| `POST` | `/api/rescan` | Force Radarr + Sonarr rescan |

---

## Development

```bash
# Clone
git clone https://github.com/cleeryy/clarr
cd clarr

# Install dependencies
go mod download

# Run locally (dry-run by default)
cp config.example.yaml config.yaml
go run ./cmd/clarr/main.go

# Run tests
go test ./...

# Build
go build -o clarr ./cmd/clarr/main.go
```

---

## License

MIT — see [LICENSE](LICENSE)
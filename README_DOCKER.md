Packaging the Maps Scraper web app in Docker (Linux)

Overview

This repository contains a Go web server and two helper binaries used by the scraping pipeline. The included `Dockerfile` builds the Go binaries and packages a minimal Debian runtime (with Chromium) so the pipeline can run inside a container.

The image exposes port 8080 and saves generated CSV files in the container's working directory. We recommend mounting a host directory to `/app` so you can easily access the generated CSVs.

Build and run (simple)

1. Set your Mapbox token in the environment:

```bash
export MAPBOX_TOKEN="pk.your_mapbox_token_here"
```

2. Build and run with docker-compose (recommended):

```bash
docker-compose build
docker-compose up -d
```

3. Open the web UI at http://localhost:8080/web/

Run with docker (manual)

```bash
docker build -t mapsscrap:latest .
# create a data directory to persist CSV outputs
mkdir -p ./data
# run the container and mount ./data so CSVs are written to the host
docker run -d --name mapsscrap -p 8080:8080 -e MAPBOX_TOKEN="$MAPBOX_TOKEN" -v $(pwd)/data:/app mapsscrap:latest
```

Notes and tips

- The Docker image includes Chromium so the phone-extraction pipeline (which may use a headless browser) can run.
- On some Linux hosts Chromium needs extra flags or /dev/shm adjustments. If you encounter rendering or sandbox errors, try passing `--shm-size=1g` to `docker run` or add `security_opt: - seccomp:unconfined` in the compose file.
- The container writes CSV files to `/app`. Mount a host directory to that path to persist output and inspect files as they are created.
- To pass a hardcoded mapbox token without environment variables, edit `web_server.go` and set `embeddedMapboxToken`.

Debugging

- Tail logs:

```bash
docker logs -f mapsscrap
```

- Attach a shell (for debugging):

```bash
docker exec -it mapsscrap /bin/bash
```

Security

- The server currently allows any origin for WebSocket upgrades for development convenience. Consider tightening CORS and the WebSocket origin check before exposing the service publicly.

License

See LICENSE in the repository root.

# Deploy guide for Render (gmaps-web-app)

This document lists exact commands to build, test locally with Docker, and push to Render.

1) Prepare local repo

```bash
# Ensure changes are committed
git status
git add Dockerfile render.yaml pipeline.sh .gitignore .dockerignore README_DEPLOY.md
git commit -m "Add Dockerfile, Render config and deploy docs"
```

2) Optional: don't push compiled binaries

```bash
# Ensure binaries are ignored
git rm --cached mapsscrap-1 web_server || true
```

3) Test locally with Docker (recommended)

```bash
# Build image
docker build -t gmaps-web-app:local .

# Run container (maps web UI on /web/)
docker run -d --name gmaps-local -p 8080:8080 gmaps-web-app:local

# Check chrome exists inside
docker exec -it gmaps-local sh -c "command -v google-chrome || command -v google-chrome-stable || echo 'no-chrome'"

docker exec -it gmaps-local sh -c "google-chrome --version || google-chrome-stable --version"

# Check web server
curl -v http://localhost:8080/web/

# Stop and remove
docker stop gmaps-local && docker rm gmaps-local
```

4) Push to GitHub and deploy on Render

```bash
git push origin main
```

Render will auto-deploy because `render.yaml` has `autoDeploy: true`. Watch the build logs in Render dashboard.

5) Troubleshooting

- If Chrome installation fails in build logs: consider using a larger plan or check network access; switch to a base image that includes Chrome.
- If binary compilation fails: verify `go mod tidy` and module dependencies.
- If you see `Exec format error` ensure you are not committing precompiled macOS binaries.


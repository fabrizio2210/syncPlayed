# Jellyfin Played Sync

Small CLI tool to synchronize "played" status of videos between two Jellyfin servers.

Usage

Build:

```bash
go build -o syncplayed
```

Run:

```bash
./syncplayed \
  -a-host HOST1 -a-user 5bb0f7e0c68448649c8e01bf2eb130d7 -a-token TOKEN_A_USER \
  -b-host HOST2 -b-user 12a69eaf3c544382ad4e6bc24830b8ed -b-token TOKEN_B_USER \
  -dry-run=true
```

- By default `-dry-run=true` so the program will only print the actions it would take.
- To actually mark items as played, pass `-dry-run=false`.

How it works

- Fetches played items for the given user from each server's `/Users/{userId}/Items` endpoint.
- For each played item on server A it searches server B using `/Items?userId=...&searchTerm=<name>`.
- If a matching item is found on the other server and it is not marked as played, it issues a `POST /UserPlayedItems/{itemId}?userId=...` to mark it played.
- Then repeats the process in the opposite direction.

Notes & Limitations

- The search uses the item's `Name` as the search term and prefers matches with the same `Id`, then exact name, then runtime.
- The tool assumes the provided tokens are valid and have permission to read items and mark user played status.
- Adjust the `IncludeItemTypes` query or other parameters in `main.go` if you want to include TV episodes or other types.

License

Use as you wish.

## Docker / CI

This repository includes a Dockerfile and a small CI script to produce images and push them to Docker Hub.

Build the image locally (x86_64):

```bash
docker build -t fabrizio2210/syncplayed:x86_64 -f docker/x86_64/Dockerfile-syncplayed .
```


Run the image (example using environment variables):

```bash
docker run -d --name syncplayed \
  -e A_HOST=HOST1 -e A_USER=5bb0f7e0c68448649c8e01bf2eb130d7 -e A_TOKEN=TOKEN_A_USER \
  -e B_HOST=HOST2 -e B_USER=12a69eaf3c544382ad4e6bc24830b8ed -e B_TOKEN=TOKEN_B_USER \
  -e DRY_RUN=true \
  fabrizio2210/syncplayed:x86_64
```

The container runs a cron daemon and executes the sync once a day (configured in `docker/syncplayed.cron`). Job stdout/stderr are written to the container's stdout and can be collected by your Docker logging driver (e.g., Docker Swarm, journald, or a log collector).

CI/CD

There is a `CICD.sh` script that follows your existing pattern — it builds the Docker image and pushes it. It expects an environment variable `DOCKER_LOGIN` with Docker Hub credentials (base64 auth value) and will tag the image with the detected architecture (`x86_64` or `armv7hf`). Example usage:

```bash
chmod +x CICD.sh
DOCKER_LOGIN="<base64-auth>" ./CICD.sh
```

Docker Compose

Example `docker-compose.yml` to run the container and supply credentials as environment variables:

```yaml
version: '3.8'
services:
  syncplayed:
    image: fabrizio2210/syncplayed:x86_64
    restart: unless-stopped
    environment:
      A_HOST: "HOST1"
      A_USER: "5bb0f7e0c68448649c8e01bf2eb130d7"
      A_TOKEN: "TOKEN_A_USER"
      B_HOST: "HOST2"
      B_USER: "12a69eaf3c544382ad4e6bc24830b8ed"
      B_TOKEN: "TOKEN_B_USER"
      DRY_RUN: "true"

```

Notes:

- The cron job inside the container runs the helper `run-sync.sh`, which reads environment variables and invokes the `syncplayed` binary.
- Keep your tokens secret: in production, use a secret manager or Docker secrets instead of embedding tokens in the compose file.

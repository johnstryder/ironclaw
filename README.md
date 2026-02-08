# Ironclaw

Agent framework (Go).

## Build

```bash
go build -o ironclaw ./cmd/ironclaw
```

## Run

```bash
./ironclaw
```

Config: `ironclaw.json` in the current directory, or set `IRONCLAW_CONFIG` to a path.

## Testing

```bash
go test ./...
```

Coverage: `go test ./... -coverprofile=cover.out` reports 100%. Optional: `go test -tags=excludemain ./... -coverprofile=cover.out` excludes the real `main`/signal code from the build for a smaller test binary.


---

## Hosting

Right now the binary prints a startup banner and exits. When the gateway is implemented it will listen on the port in config (e.g. `:8080`). You can host it in these ways.

### 1. Direct run (VPS / your machine)

```bash
./ironclaw
```

To keep it running in the background (e.g. until you add a real server loop):

```bash
nohup ./ironclaw >> ironclaw.log 2>&1 &
```

Or run it inside a process manager (see below).

### 2. systemd (Linux)

Create `/etc/systemd/system/ironclaw.service`:

```ini
[Unit]
Description=Ironclaw agent framework
After=network.target

[Service]
Type=simple
User=ironclaw
WorkingDirectory=/opt/ironclaw
ExecStart=/opt/ironclaw/ironclaw
Restart=on-failure
RestartSec=5
Environment=IRONCLAW_CONFIG=/opt/ironclaw/ironclaw.json

[Install]
WantedBy=multi-user.target
```

Then:

```bash
sudo systemctl daemon-reload
sudo systemctl enable ironclaw
sudo systemctl start ironclaw
```

### 3. Docker

Build and run:

```bash
docker build -t ironclaw .
docker run --rm -p 8080:8080 -v $(pwd)/ironclaw.json:/app/ironclaw.json ironclaw
```

To run as a long-lived container:

```bash
docker run -d --name ironclaw -p 8080:8080 -v $(pwd)/ironclaw.json:/app/ironclaw.json --restart unless-stopped ironclaw
```

### 4. Cloud / PaaS

- **Fly.io**: `fly launch` in the repo (after `flyctl` is installed), then add a `fly.toml` that runs the built binary.
- **Railway / Render**: Connect the repo, set build to `go build -o ironclaw ./cmd/ironclaw` and start command `./ironclaw`. Mount or set `IRONCLAW_CONFIG` if needed.

For all of the above, ensure `ironclaw.json` (and any `agents/` / `memory/` paths in it) are available in the working directory or adjust paths in config.

---

## CLI

| Command | Description |
|--------|-------------|
| `./ironclaw` | Start (banner, load config, block until SIGINT/SIGTERM). |
| `./ironclaw check` | Health check: config, gateway, paths. |
| `./ironclaw check --fix` | Same as above; creates default `ironclaw.json` if missing. |
| `./ironclaw 💅🏼` | Prints the nail polish emoji and exits. |


Possible go projects:
| Category | Go Package | Implementation Notes |
| :--- | :--- | :--- |
| **Brain (AI)** | `github.com/tmc/langchaingo` | Standard Go port of LangChain. Stricter than JS; prevents runtime chain failures. |
| **Brain (AI)** | `github.com/aws/aws-sdk-go-v2` | Modular AWS SDK. Import only `service/bedrockruntime` to keep binary size small. |
| **Brain (AI)** | `github.com/go-skynet/go-llama.cpp` | Bindings for `llama.cpp`. Runs GGUF models directly inside the binary (no server needed). |
| **Brain (AI)** | `github.com/jmorganca/ollama/api` | Import Ollama's API client natively (since Ollama is written in Go). |
| **Brain (AI)** | `github.com/invopop/jsonschema` | Auto-generates JSON Schemas from Go structs for LLM function calling. |
| **Brain (AI)** | `github.com/asg017/sqlite-vec-go-bindings` | Embeds Vector Search C-extension directly. No external Python/C toolchain required. |
| **Mouth (Msg)** | `github.com/tulir/whatsmeow` | Industry standard for WhatsApp. Reverse-engineered from Android protocol. |
| **Mouth (Msg)** | `github.com/bwmarrin/discordgo` | Mature, stable Discord library. Handles sharding/reconnection automatically. |
| **Mouth (Msg)** | `github.com/go-telegram-bot-api/telegram-bot-api` | Simple wrapper. Use long-polling for local dev, webhooks for production. |
| **Mouth (Msg)** | `github.com/slack-go/slack` | Comprehensive Slack API coverage, including Block Kit construction. |
| **Mouth (Msg)** | `github.com/line/line-bot-sdk-go` | Official SDK maintained by LINE. |
| **Mouth (Msg)** | `github.com/gorilla/websocket` | Gold standard for WebSockets. Handles 10k+ concurrent connections efficiently. |
| **Eyes (Web)** | `github.com/chromedp/chromedp` | Controls Chrome via CDP. Faster/lighter than Puppeteer (no Node bridge). |
| **Eyes (Web)** | `github.com/PuerkitoBio/goquery` | "jQuery for Go." Blazing fast HTML parsing (no DOM simulation overhead). |
| **Eyes (Web)** | `github.com/go-shiori/go-readability` | Port of Mozilla's Readability.js. Critical for extracting article text. |
| **Eyes (File)** | `github.com/disintegration/imaging` | Pure Go image processing (resize, crop). Safer than ImageMagick bindings. |
| **Eyes (File)** | `github.com/h2non/filetype` | Dependency-free file type detection via magic numbers. Matches Node logic. |
| **Eyes (File)** | `github.com/yuin/goldmark` | Fastest Markdown parser in Go. Used by Hugo and GitHub. |
| **Infra** | `github.com/joho/godotenv` | Reads `.env` files. Optional, but keeps dev parity with Node workflows. |
| **Infra** | `github.com/spf13/cobra` | Standard for building CLI apps (like Kubernetes/Docker). Handles flags/commands. |
| **Infra** | `github.com/robfig/cron` | Robust cron scheduler. Essential for agents running "Daily Audits". |
| **Infra** | `github.com/mdp/qrterminal` | Renders QR codes in the terminal (for WhatsApp pairing). |
| **Infra** | `net/http` (Std Lib) | Production-ready HTTP server. Replaces Express/Hono entirely. |
| **Infra** | `log/slog` (Std Lib) | Structured, JSON-capable logging. Replaces Winston/Tslog. |
# Kexoz CLI

**Share your local apps securely — from the command line.**

Headless, single-binary client for [Kexoz](https://kexoz.dev), built and maintained by
[Wrenyn](https://wrenyn.com). Share web apps and raw TCP/UDP services through an encrypted,
multiplexed tunnel. For feedback, revisions and a visual dashboard, use
[Kexoz Desktop](https://github.com/wrenyn/kexoz-desktop).

```console
$ kexoz http 3000
Forwarding  https://my-app.kexoz.dev -> http://localhost:3000
Inspector    http://localhost:4040
```

## What you get

- **HTTP tunnels** with a stable `https://<subdomain>.kexoz.dev` URL
- **Private (E2E) tunnels** — TLS terminates on your machine, the gateway only relays bytes
- **Raw TCP / UDP** for Postgres, SSH, game servers, DNS
- **Path routing** (`--route /api=7000`), `--password` basic auth, `--one-time`, `--duration 1h`
- **Saved projects**: `kexoz start my-project` brings up every share in a project at once
- **Request inspector** on `http://localhost:4040` to view and replay incoming requests

## Install

| Method | Command |
| --- | --- |
| macOS / Linux | `curl -fsSL https://kexoz.dev/install.sh \| sh` |
| Windows | `irm https://kexoz.dev/install.ps1 \| iex` |
| Homebrew | `brew install wrenyn/tap/kexoz` |
| Chocolatey | `choco install kexoz` |
| npm wrapper | `npm install -g @kexoz/cli` |
| Container (GHCR) | `docker run --rm --network host -it ghcr.io/wrenyn/kexoz:latest http 3000` |
| Prebuilt binaries | [releases](https://github.com/wrenyn/kexoz-cli/releases) — windows / linux / macOS × amd64 / arm64 |

Verify the install with `kexoz version` (or `kexoz --version`). The installer scripts
honour `KEXOZ_CLI_REPO` and `KEXOZ_INSTALL_DIR` to install from another release source or directory.

## Quick start

```bash
kexoz login              # link your Kexoz account (unlocks the full free plan)
kexoz http 3000          # expose localhost:3000 over HTTPS
```

## Commands

| Command | Description |
| --- | --- |
| `kexoz http [port]` | HTTP tunnel. Flags: `--subdomain`, `--route /path=port`, `--password`, `--duration 1h`, `--one-time`, `--tcp` |
| `kexoz e2e [port]` | End-to-end encrypted tunnel (TLS terminated locally). Flag: `--subdomain` |
| `kexoz tcp [port]` | Raw TCP tunnel (SSH, Postgres, …) on a public port |
| `kexoz udp [port]` | UDP tunnel (DNS, game servers). Flag: `--tcp` |
| `kexoz start [project]` | Start a saved project's shares and print their links (uses `kexoz.json` when no project is named). Flag: `--env staging` |
| `kexoz init [project]` | Write a `kexoz.json` linking this folder to a project. Flags: `--env`, `--force` |
| `kexoz projects` | List saved projects and their environments |
| `kexoz env new <name> [project]` | Copy an environment's shares (main by default, `--from`) into a new preview environment |
| `kexoz inbox [project]` | Show the project's webhook inbox URL and latest requests. Flag: `--create` |
| `kexoz inbox replay <request-id>` | Send a stored inbox request to your app again. Flag: `--share` |
| `kexoz connect [tunnel-id]` | Dial a remote raw TCP tunnel locally. Flag: `--port` (default 8080) |
| `kexoz tunnel list \| create \| delete \| routes` | Manage persistent tunnel configurations (`create`: `--subdomain`, `--port`, `--route`) |
| `kexoz domain claim [subdomain] \| custom [domain]` | Claim a subdomain or map a custom domain (Pro) |
| `kexoz login [--token] \| logout \| status \| version` | Identity management |

Global flags: `--server` / `-s`, `--root-domain` / `-d`, `--json` for machine-readable output.

The default gateway is `https://api.kexoz.dev`. Point it at your own deployment with
`kexoz --server https://api.example.com --root-domain example.com http 3000`, or set
`KEXOZ_SERVER_URL` / `KEXOZ_ROOT_DOMAIN`.

## Projects in a folder

Run `kexoz init Shop` once in a repository and commit the `kexoz.json` it writes. Anyone on
the project can then run `kexoz start` from that folder (or any folder below it) to start
every share and print the links:

```json
{
  "project": "Shop",
  "environment": "staging",
  "ports": { "web": 5173, "web /api": 8000 }
}
```

- `environment` picks which copy of the project to start; leave it out for main.
  `kexoz start --env pr-12` overrides it for one run.
- `ports` is optional. It changes the local port for a share (by name or address), or for one
  of its path routes with `"<share> /path"`, for people whose app runs elsewhere.
- Path routes from the project (for example `/api` to a backend) are honoured, so one
  address can serve both the frontend and the backend.

Create a preview environment with `kexoz env new staging`. Each copy gets its own address and
keeps the original's ports, routes and access rules.

The project's webhook inbox (`kexoz inbox --create`) has a permanent URL. Kexoz stores every
request sent to it even while your app is offline, forwards them live while the inbox's share
is running, and `kexoz inbox replay <id>` resends one.

## Configuration

The client identity and plan are stored in `~/.config/kexoz/config.yaml` (the legacy
`~/.kexoz/config.json` is still read for migration). `kexoz logout` unlinks the account
without deleting the client identity.

## Transports

By default the agent connects over a WSS WebSocket and multiplexes every stream (HTTP, TCP, UDP)
with [yamux](https://github.com/hashicorp/yamux). On networks that block WebSockets, pass
`--tcp <server:8081>` to use the raw TCP agent transport instead.

## Security

- The client identity and the linked account live in `~/.config/kexoz/config.yaml`, which is
  written with owner-only (`0600`) permissions. For automation, issue an API token
  (`kexoz_sk_…`); using the raw client identity as a bearer credential is deprecated.
- `kexoz e2e` keeps request paths, headers and bodies opaque to the gateway.
- WebSocket upgrades are origin-checked and per-IP rate limits/bandwidth caps apply per plan tier.

See [SECURITY.md](./SECURITY.md) for the reporting policy.

## Development

```bash
go build -o kexoz .     # or: go run . http 3000
go vet ./... && go test ./...
```

| Path | Contents |
| --- | --- |
| `cmd/` | Cobra commands (one file per subcommand) |
| `tunnel/` | WebSocket + yamux transport, TCP/UDP agents, request inspector |
| `config/` | On-disk identity/config handling |
| `packages/` | Distribution wrappers: npm (`@kexoz/cli`), Chocolatey, Homebrew |

Releases are published by [`.github/workflows/release.yaml`](./.github/workflows/release.yaml):
a `v*` tag builds the six binaries, pushes `ghcr.io/<owner>/kexoz`, and publishes the npm
wrapper. See [CONTRIBUTING.md](./CONTRIBUTING.md) and [ARCHITECTURE.md](./ARCHITECTURE.md).

## License

Proprietary. © [Wrenyn](https://wrenyn.com), all rights reserved; see [LICENSE](./LICENSE)

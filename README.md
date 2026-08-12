# vpnserver-manager

Per-VPS **Go HTTP agent** (`goserver`) plus an **IPsec compose bundle** for Mobify VPN nodes.

## Role

This repository is the **node-side** control plane for a single VPN server:

- Runs on each VPS next to IPsec (and optionally OpenVPN on the host).
- Exposes a small Bearer-authed HTTP API on **`:8080`**.
- Mutates local VPN credential files and drives Docker (IPsec container / helpers).

It is **not** the desktop local service (`vpn-win` on `:8090`).  
It is **not** the central `socket-server` inside `vpnpro_main` (that process may also bind `:8080`, but on a different host and with a different role).

## Call direction

```
vpnpro_main (Laravel / central) ──HTTP──► this agent on the node (:8080)
```

- **`vpnpro_main` calls this agent.** The agent does **not** call Laravel.
- The agent talks outbound only to the local Docker Engine (via mounted `docker.sock`) and local files.

## Layout

| Path | What it is |
| -- | -- |
| [`goserver/`](goserver/) | Chi HTTP agent source + Dockerfile |
| [`ipsec/`](ipsec/) | Vendored upstream IPsec-on-Docker tree (hwdsl2 lineage) |
| [`docker-compose.yml`](docker-compose.yml) | Runtime compose: `ipsec-mobify-server` + `goserver` |
| [`start.sh`](start.sh) | Host bootstrap: create `./etc/**` stubs, `docker compose up -d` |
| [`docs/DEPLOY.md`](docs/DEPLOY.md) | Deploy checklist (host files, images, caveats) |
| `prod.yml` | **Gitignored** agent config mounted into the container |
| `./etc/**` | **Gitignored** host-persisted `chap-secrets` / `passwd` / `ipsec.secrets` |

## Auth

All HTTP routes require:

```http
Authorization: Bearer <token>
```

The token must equal the YAML key **`auth_password`** in the config file pointed to by **`CONFIG_PATH`** (compose uses `/app/default.yml` ← host `prod.yml`).

Do not commit real `prod.yml` or paste secret values into docs.

## API summary (`master`)

| Method | Path | Purpose | Risk |
| -- | -- | -- | -- |
| GET | `/api/v1/users` | List IPsec/L2TP usernames (includes PSK from secrets file) | Medium |
| POST | `/api/v1/users` | Create users; server generates passwords | High |
| POST | `/api/v1/restart/container` | Restart Docker container `ipsec-mobify-server` | High |
| POST | `/api/v1/restart/service` | `ipsec restart` inside that container | High |
| POST | `/api/v1/exec` | Arbitrary docker exec (`container` + `command[]`) | **Critical** |
| GET | `/api/v1/version` | Returns API version integer (`1`) | Low |
| GET | `/api/v1/openvpn/profiles` | List `/root/*.ovpn` (optional full content) | High |
| POST | `/api/v1/openvpn/profiles` | Create client via host `openvpn.sh` | High |
| DELETE | `/api/v1/openvpn/profiles` | Revoke + delete profile | High |

Full route notes, auth quirks, and side effects: [`goserver/README.md`](goserver/README.md).

### Not in this git tree

- **No UDP `:8081` ping listener**
- **No `/api/v1/bandwidth/*` routes**

Live VPS nodes may still run sibling builds / Hub tags such as `lmobify/goserver:bw-*` that *do* expose bandwidth HTTP APIs and UDP `:8081`. **Do not equate this GitHub SHA with what is running on a node.**

## Deploy (short)

Root compose pulls **prebuilt** images:

- `lmobify/goserver:latest`
- `lmobify/ipsec-mobify:latest`

`goserver` mounts **`/var/run/docker.sock`**, so a compromised agent bearer token is effectively host-root via Docker.

Bootstrap and host file requirements: [`docs/DEPLOY.md`](docs/DEPLOY.md).

## OpenVPN assumptions

OpenVPN profile handlers expect:

- Host script at **`/root/openvpn.sh`**
- Profiles under **`/root/*.ovpn`**
- Helpers that spawn a privileged ephemeral container with binds for `/root` and `/etc/openvpn`

Stock root compose does **not** mount `/root` or `/etc/openvpn` into the `goserver` container. File checks that run inside the agent process may therefore miss host profiles unless goserver runs on the host or extra mounts are added. See [`goserver/README.md`](goserver/README.md) and [`docs/DEPLOY.md`](docs/DEPLOY.md).

## Security highlights

| Issue | Why it matters |
| -- | -- |
| `POST /api/v1/exec` | Unrestricted command execution in a chosen container |
| `docker.sock` mount | Agent can control the Docker daemon on the host |
| Auth failure logging | Middleware logs the **provided** Bearer token on mismatch |
| `ListenAndServe` | No TLS in-process; rely on network controls / reverse proxy |
| Static `auth_password` | Long-lived shared secret = full admin over every route |

Treat the agent as a privileged management endpoint. Restrict network access to `vpnpro_main` (or an equivalent control plane) only.

## Parts

| Doc | Contents |
| -- | -- |
| [`goserver/README.md`](goserver/README.md) | Agent routes, auth, local side effects, propose-only hardenings |
| [`docs/DEPLOY.md`](docs/DEPLOY.md) | `start.sh`, compose, host files, image tags, `ipsec/` ownership |
| [`ipsec/README.md`](ipsec/README.md) | Upstream IPsec image docs (+ Mobify note at top) |

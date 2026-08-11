# Deploy (node agent + IPsec)

How this repo is meant to run on a VPS. Secrets stay on the host — never commit `prod.yml` or real files under `./etc/`.

## Quick start

From the repo root on the node:

```bash
bash start.sh
```

`start.sh`:

1. Creates `etc/ipsec.d/` and `etc/ppp/` if missing.
2. Touches empty `etc/ipsec.d/passwd`, `etc/ppp/chap-secrets`, `etc/ipsec.secrets`.
3. Runs `docker compose up -d`.

### `start.sh` shebang

The first line is `# !/bin/sh` (space after `#`), so it is **not** a valid shebang. Invoke with:

```bash
bash start.sh
```

(or fix the shebang locally). Do not rely on `./start.sh` unless corrected.

## Required host files (gitignored)

| Host path | Mounted as (compose) | Role |
| -- | -- | -- |
| `./prod.yml` | `/app/default.yml` in `goserver` | Agent YAML (`CONFIG_PATH=/app/default.yml`) |
| `./etc/ppp/chap-secrets` | `/etc/ppp/chap-secrets` | L2TP/IPsec user lines |
| `./etc/ipsec.d/passwd` | `/etc/ipsec.d/passwd` | XAuth password hashes |
| `./etc/ipsec.secrets` | `/etc/ipsec.secrets` | PSK material (agent **reads** PSK; does not rewrite this file on user create) |

`prod.yml` must define at least: `env`, `storage_path`, `auth_password`, `logfile_path`, `http_server.address`.  
With the stock mounts, `storage_path` should point at the directory that contains `ppp/`, `ipsec.d/`, and `ipsec.secrets` inside the container (typically `/etc`).

Do not paste real `auth_password` values into git or tickets.

## Compose services

Root [`docker-compose.yml`](../docker-compose.yml):

| Service | Image (as committed) | Ports | Notes |
| -- | -- | -- | -- |
| `ipsec-mobify-server` | `lmobify/ipsec-mobify:latest` | `500/udp`, `4500/udp` | Privileged; modules + secret file mounts |
| `goserver` | `lmobify/goserver:latest` | `8080` | Mounts **`/var/run/docker.sock`**, `prod.yml`, same secret files |

Images are **prebuilt Hub tags**. Root compose does **not** use `build: ./goserver` or `build: ./ipsec`.

### Image tags in the wild

This repo pins `:latest`. Other tags may appear on live nodes (for example `lmobify/goserver:bw-*` / matching IPsec tags) that add bandwidth HTTP APIs and UDP `:8081`. Those features are **absent from this git tree**. Always check the running container image/tag on the VPS before assuming API shape.

## What `ipsec/` is

[`ipsec/`](../ipsec/) is a **vendored upstream** IPsec-on-Docker tree (hwdsl2 lineage: Dockerfiles, `run.sh`, helper scripts, upstream docs/CI).

- Useful as reference / optional local build source.
- **Root compose runs `lmobify/ipsec-mobify:latest`**, not necessarily `build: ./ipsec`.
- Nested `ipsec/docker-compose.yml` is upstream’s example (`hwdsl2/ipsec-vpn-server`, different container name) and is **not** the Mobify node stack wired to `goserver`.

Mobify-facing note also lives at the top of [`ipsec/README.md`](../ipsec/README.md).

## OpenVPN on the host

If you use the agent’s OpenVPN profile routes:

- Expect **`/root/openvpn.sh`** on the **host** (this Mobify tree does not ship that script at repo root).
- Profiles are expected at **`/root/*.ovpn`**.
- Stock `goserver` service does **not** bind-mount `/root` or `/etc/openvpn`. The helper can still mutate host paths via a privileged sibling container, but in-process file checks inside `goserver` may not see host `/root`. Prefer documenting extra mounts or running the agent where those paths are shared intentionally.

## Network / security checklist

- Keep `:8080` reachable only from `vpnpro_main` (or equivalent), not the public internet.
- Bearer token = YAML `auth_password`; rotate on compromise.
- Treat `POST /api/v1/exec` and `docker.sock` as critical risk (see [`goserver/README.md`](../goserver/README.md)).
- Agent uses plain `ListenAndServe` (no in-process TLS).

## Related docs

- [Root README](../README.md) — role, topology, API map
- [goserver README](../goserver/README.md) — routes and side effects

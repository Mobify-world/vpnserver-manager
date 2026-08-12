# goserver

Chi-based HTTP agent that runs on each VPN node. Module path: `github.com/LevanPro/server`.

See the [root README](../README.md) for topology and call direction (`vpnpro_main` → this agent on `:8080`).

## Runtime

- Entry: `cmd/server` → `http.ListenAndServe` on `http_server.address` (YAML default `:8080`).
- Config: set env **`CONFIG_PATH`** to a YAML file. Compose mounts host `prod.yml` as `/app/default.yml` and sets `CONFIG_PATH=/app/default.yml`.
- Required YAML keys (names only): `env`, `storage_path`, `auth_password`, `logfile_path`, `http_server.address`.
- Sample committed file: `config/default.yml` (dev sample only — never use its secrets in production).

## Auth middleware

- Header: `Authorization: Bearer <token>`.
- Token must equal YAML **`auth_password`**.
- Applied to **every** route (including `/api/v1/version`).
- **Quirk:** failures return **HTTP 400** (not 401/403).
- **Quirk:** on mismatch, the middleware logs the **provided** token string.

## Routes (`master`)

| Method | Path | Purpose | Risk |
| -- | -- | -- | -- |
| GET | `/api/v1/users` | Read usernames from `{storage_path}/ppp/chap-secrets`; attach PSK from `{storage_path}/ipsec.secrets` | Medium |
| POST | `/api/v1/users` | JSON array of users; generate passwords; append chap-secrets + `{storage_path}/ipsec.d/passwd` | High |
| POST | `/api/v1/restart/container` | Docker restart of hard-coded container `ipsec-mobify-server` | High |
| POST | `/api/v1/restart/service` | Privileged docker exec: `ipsec restart` in that container | High |
| POST | `/api/v1/exec` | Body `{"container","command"}` — arbitrary docker exec | **Critical** |
| GET | `/api/v1/version` | `{"data":{"version":1}}` | Low |
| GET | `/api/v1/openvpn/profiles` | List `/root/*.ovpn`; `?include_content=true` includes file bodies | High |
| POST | `/api/v1/openvpn/profiles` | Body `{"client_name"}` → `bash /root/openvpn.sh --addclient …` | High |
| DELETE | `/api/v1/openvpn/profiles` | Body `{"client_name"}` → `--revoke` + delete `.ovpn` (blocks deleting default `client`) | High |

### Not in this tree

No UDP `:8081` service and no `/api/v1/bandwidth/*` handlers. Sibling / `lmobify/goserver:bw-*` images may differ — see [root README](../README.md).

## Side effects

| Area | Behavior |
| -- | -- |
| Credential files | Read/append under `storage_path` (`ppp/chap-secrets`, `ipsec.d/passwd`, read-only use of `ipsec.secrets` for PSK) |
| Password hashing | Host/container `openssl passwd -1` for IPsec passwd entries |
| Docker | Engine API via env/socket; hard-coded container name `ipsec-mobify-server` for restart paths |
| OpenVPN helpers | Creates privileged `ubuntu:latest` with binds `/root:/root`, `/etc/openvpn:/etc/openvpn`, `PidMode=host`, then runs `/root/openvpn.sh` |
| OpenVPN file IO | `os.Stat` / `ReadFile` / `Remove` / `Glob` against `/root` **inside the goserver process** |

Compose binds chap-secrets / passwd / secrets under `/etc/...`. Production `storage_path` must match those mounts (typically `/etc` in host `prod.yml`).

OpenVPN: root compose does not mount `/root` into `goserver`, so host profile create/list/delete can fail or see the wrong filesystem when the agent runs as a container. Details: [`docs/DEPLOY.md`](../docs/DEPLOY.md).

## Propose-only hardenings

Documentation only — not implemented here:

1. **Remove or gate `/api/v1/exec`** (allowlist container + argv, or delete from production builds).
2. **Harden auth:** return 401 on failure; never log Bearer material; rotate `auth_password` via a secret store; prefer mTLS or IP allowlist from `vpnpro_main`.
3. **TLS / network:** do not expose plain `:8080` on the public internet; terminate TLS at a proxy or private management network.
4. **Docker socket:** replace raw `docker.sock` with a restricted socket proxy where possible.
5. **OpenVPN path:** run helpers without privileged ephemeral containers, or mount the needed host paths deliberately and document them in compose.

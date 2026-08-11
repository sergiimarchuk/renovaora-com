# Secure Web Project — Architecture Plan

## Deployment / Server State — Important Ongoing Notes

**Repo sync status (as of this session):**
- Server (`ubuntu@main-net:/opt/web`) and laptop (`sergii@smu-22:/opt/web/renovaora-com`) are both fully synced to the same commit, containing all auth logic, admin Dockerfile/start.sh, and `docker-compose.yml`
- **Important distinction:** on the laptop, `/opt/web` itself is a stray, unrelated orphaned git repo (no tracking branch) — the real project repo is one level deeper, at `/opt/web/renovaora-com`. Always `cd` into the `renovaora-com` subfolder on the laptop before running git commands.
- `renovaora-com.backup-m1/` still sits in `/opt/web` on the server, excluded via `.gitignore` — harmless, not yet decided whether to delete it entirely

**Docker tooling on the server — confirmed:**
- The server has the **older standalone `docker-compose`** (v1.29.2, hyphenated command), **not** the newer `docker compose` (v2, space-separated) plugin
- Use `docker-compose --profile admin build admin` / `docker-compose --profile admin up -d admin`, not `docker compose ...`
- Uses the legacy (non-BuildKit) builder — works, but shows a deprecation warning; upgrading to the `docker compose` v2 plugin + BuildKit is a worthwhile future improvement, not urgent

**NB — server is underpowered for Docker builds; build on laptop and transfer instead:**
- The server is a modest cloud VM and Go/Docker builds run noticeably slowly on it
- **Recommended workflow going forward:** build images on the laptop, then transfer to the server, rather than building directly on the server
  ```bash
  # On laptop:
  cd /opt/web/renovaora-com
  docker build -t renovaora-admin:latest ./admin
  docker save renovaora-admin:latest | gzip > renovaora-admin.tar.gz
  scp renovaora-admin.tar.gz ubuntu@main-net:/opt/web/

  # On server:
  cd /opt/web
  gunzip -c renovaora-admin.tar.gz | docker load
  ```
- `docker-compose.yml` would then reference `image: renovaora-admin:latest` for that service instead of `build: context: ./admin`
- **Important prerequisite:** laptop and server CPU architecture must match (`uname -m` on both — expect `x86_64` for this Oracle Cloud server). If they differ (e.g. Apple Silicon laptop), use `docker buildx build --platform linux/amd64 ...` to cross-compile for the server's architecture instead
- Not yet adopted as the standard workflow — still building directly on the server as of this session, but worth switching to for future builds

**Environment file (`.env`) — done:**
- Created on the server at `/opt/web/.env` (gitignored, not committed)
- `ADMIN_DB_USER=appnutzer`
- `ADMIN_DB_PASSWORD` — generated via `openssl rand -hex 32` (hex-only output, deliberately **not** `openssl rand -base64`, since base64 can include `/`, `+`, `=` characters that break URL parsing inside `DATABASE_URL`)

**Common pitfalls hit during setup (for future reference):**
- Pasting multi-line commands with a trailing `#` comment on the last line can swallow a closing `)`, leaving the shell stuck on a continuation prompt (`>`) — remove inline comments from pasted multi-line shell blocks, or press `Ctrl+C` to cancel if it happens
- Running `go build` without `-o` writes the compiled binary into the repo directory itself, which can then get accidentally `git add`-ed — always build to a throwaway path (e.g. `-o /tmp/admin-test-build`) when just testing compilation, and keep `admin/admin` / `public/public` in `.gitignore` as a safety net
- Double-check the current working directory (`pwd`) before running `tar -xzf ... --strip-components=1` or any `git` command — several mix-ups in this session came from running commands in the wrong directory (e.g. an unrelated `TimeTracking-dev` folder, or the stray outer `/opt/web` repo on the laptop)

**Current status:** first `docker-compose --profile admin build admin` run in progress — image build started successfully (Go base layers pulling), not yet confirmed complete.

## Implementation Progress

**Repo:** `github.com/sergiimarchuk/renovaora-com` (private), server working dir `/opt/web`, laptop working dir `/opt/web/renovaora-com`.

**Folder structure created:**
```
renovaora-com/
├── public/
│   ├── templates/
│   ├── migrations/
│   ├── Dockerfile
│   └── start.sh
├── admin/
│   └── migrations/
└── media/
    ├── staging/
    └── live/
```

**Public container — done:**
- **`public/Dockerfile`** — multi-stage build:
  - Build stage: compiles the Go app as a static binary (`CGO_ENABLED=0`), also compiles the `golang-migrate` CLI into `/app/migrate`
  - Runtime stage: Alpine base with `postgresql16` installed directly in the image (bundled DB, per the architecture decision to keep this a 2-container project), plus `su-exec` for privilege dropping
  - Creates a non-root OS user **`appnutzer`** (uid 10001) — the Go app never runs as root or as the `postgres` user
  - `EXPOSE 8111`, entrypoint is `start.sh`
- **`public/start.sh`** — container entrypoint, runs in this order:
  1. Initializes the Postgres data directory on first run only (`scram-sha-256` auth, password passed via temp file so it never appears in `ps aux`)
  2. Locks Postgres to `listen_addresses = 'localhost'` — not reachable from outside this container at all, since DB and app are bundled together
  3. Starts Postgres as the `postgres` system user
  4. Runs schema migrations automatically via the bundled `migrate` binary
  5. Drops privileges and execs the Go binary as `appnutzer`
  - Requires `POSTGRES_PASSWORD` and `DATABASE_URL` env vars at startup (to be wired via `.env` + `docker-compose.yml`, never hardcoded — not yet built)

**Auth DB schema — done:**
- `admin/migrations/001_create_users_table` — `users` table (id, username, password_hash, created_at, deleted_at) with a partial unique index on `username` that ignores soft-deleted rows
- `admin/migrations/002_create_sessions_table` — `sessions` table (id = session token, user_id FK with `ON DELETE CASCADE`, created_at, last_activity, expires_at), indexed on `user_id` and `expires_at`
- `admin/migrations/003_create_login_attempts_table` — `login_attempts` table (username, ip_address, success, attempted_at) for login rate-limiting/lockout, no FK to `users` (intentionally logs attempts against nonexistent usernames too)
- Each migration has matching `.up.sql` / `.down.sql` pairs, run automatically by the bundled `migrate` binary at container startup

**Admin app Go skeleton — done:**
- **`admin/go.mod`** — module `github.com/sergiimarchuk/renovaora-com/admin`, Go 1.22, dependencies: `pgx/v5` (Postgres driver) and `golang.org/x/crypto` (bcrypt)
- **`admin/main.go`**:
  - Connects to Postgres via `DATABASE_URL` env var, fails fast (`log.Fatal`) if missing or unreachable
  - stdlib `net/http` router (Go 1.22+ method-based routing) — no framework, per the tech stack decision
  - Registers `/healthz`, `/login` (GET+POST), `/logout` routes
  - Explicit server timeouts set (`ReadTimeout`, `WriteTimeout`, `IdleTimeout`) as basic hardening
  - Listens on port 8002 by default (`ADMIN_PORT` env var override)
- **`admin/handlers.go`** — stub handlers (`501 Not Implemented`) so the skeleton compiles; real logic is the next step
- **`go mod tidy`** run successfully on laptop (Go 1.22.3) — dependencies resolved and downloading (`pgx/v5`, `x/crypto`, transitive deps); `go.sum` generation confirmed in progress
- **Note:** Go is not installed on the server, and doesn't need to be — compilation happens inside the Docker build stage (`golang:1.22-alpine`) instead

**Auth logic (step 3) — done:**
- **`admin/password.go`** — bcrypt hashing (`HashPassword`) and verification (`CheckPassword`); cost factor 12 (balances brute-force resistance against responsiveness for a single, occasional admin login — 14 was considered but judged unnecessarily slow for this use case)
- **`admin/session.go`**:
  - `generateSessionToken` — 32 bytes (256 bits) of crypto-random data, base64 URL-safe encoded
  - `CreateSession` — inserts a new session row tied to a user
  - `GetValidSession` — looks up a session, auto-deletes and returns "no session" if it's past hard expiry (60 min) or past inactivity timeout (30 min); otherwise slides `last_activity` forward on legitimate use
  - `DeleteSession` — used by logout and by the expiry/timeout cleanup path
- **`admin/users.go`** — `CreateUser` (hashes + inserts, not exposed via any HTTP route — CLI/seed only, no public registration) and `GetUserByUsername` (ignores soft-deleted rows, returns nil rather than erroring if not found — avoids leaking whether a username exists)
- **`admin/rate_limit.go`** — `RecordLoginAttempt` (logs every attempt, doubles as audit trail) and `IsLockedOut` (blocks further attempts after 5 failures within 15 minutes, checked independently by both username and IP — catches both a distributed attack on one account and a single IP guessing many usernames)
- **`admin/handlers.go`** rewritten with real logic:
  - `handleLoginPage` — minimal inline HTML login form (no template engine wired in yet for the admin side)
  - `handleLoginSubmit` — checks lockout first, looks up user, verifies password, **always runs a bcrypt comparison even for nonexistent usernames** (against a dummy hash) so response timing doesn't reveal whether a username exists, records the attempt either way, creates a session and sets the cookie on success
  - Session cookie set with `HttpOnly`, `Secure`, `SameSite=Strict` — not accessible to JS, only sent over HTTPS, not sent cross-site
  - `handleLogout` — deletes the session server-side and clears the cookie
- **`admin/middleware.go`** — `AuthRequired` wrapper: checks for a valid session cookie, redirects to `/login` (or `/login?timeout=1` if the session existed but expired) if invalid, otherwise attaches `userID` to the request context and calls the wrapped handler
- **`admin/main.go`** updated — added a protected `GET /` route wrapped in `AuthRequired`, pointing at a placeholder `handleDashboard` that just confirms the auth gate works end-to-end (renders "Logged in" + a logout button)

**Not yet built:**
- Admin container's Dockerfile + start.sh (same bundled pattern as public, port 8002 / auth DB)
- `docker-compose.yml` (env var wiring, network segmentation, volumes)
- Public app's Go source code (currently only Dockerfile/start.sh exist for public container — no `.go` files or content-DB migrations yet)
- Content management CRUD (pages, articles, media, contact info) — dashboard route exists but only as a placeholder confirming the auth gate works
- One-time CLI/seed step to create the first admin user (no public registration exists by design)
- Caddy configuration file itself (plan exists in doc, not yet written)
- `go build`/compile check not yet run — files created but not yet verified to compile cleanly

## Architecture

### Container 1 — Public
- Go web app (serves public info/service pages)
- Bundled Postgres instance (public content DB) — own data volume
- Always running
- No credentials, no network path to the auth DB — enforced via a separate Docker network, not just app config

### Container 2 — Auth/Admin
- Go admin app (login, manage/insert/edit/delete content)
- Bundled Postgres instance (auth DB: users/passwords) — own data volume
- Spun up manually via SSH only when management is needed, then stopped
- Writes directly to the shared public content DB while running; publish/unpublish is a `status` flag update, not a separate sync step (see Content Flow section)

> **Note:** Bundling app + DB in the same container isn't the typical Docker pattern (one process per container is usually cleaner for restarts/backups), but it's a valid tradeoff for keeping the count at 2 — just means backup/restart logic lives inside each container instead of being handled by Compose separately.

## Network Segmentation
- `public-net` and `admin-net` — two isolated Docker networks
- Public container only on `public-net`
- Admin container only on `admin-net`, plus whatever it needs to reach the public DB for publishing content (a narrow, defined path — not open access)
- Host firewall: admin container's port bound to localhost only or reachable solely via SSH tunnel — never exposed directly to the internet
- **Web server / reverse proxy: Caddy** (not NGINX) — automatic HTTPS, simpler config, actively maintained
  - Caddy serves static files (images) directly from the shared media volume (`/media/live/`) — bypasses the Go app entirely for static assets, faster
  - Caddy also reverse-proxies dynamic requests to the public Go app (port 8111)

## Auth Design
- Password hashing: bcrypt or argon2
- **Session handling (decision):** DB-backed sessions, using a session table in the auth DB — not JWT
  - Rationale: allows instant revocation (delete session row) vs a JWT that stays valid until expiry regardless of server-side action
  - Usage pattern (single admin, low-frequency, intermittent container) doesn't benefit from JWT's statelessness/scaling advantages
  - Session table can double as a login audit trail (ties into the deferred audit log idea)
  - Short expiry (e.g. 30–60 min); sliding vs fixed expiry to be decided at implementation time
- Rate limiting / lockout on the login endpoint (e.g., exponential backoff or temp ban after N failed attempts)
- TLS via Caddy in front (deferred — added later)

## Tech Stack
- **Web framework:** none — standard library only (`net/http`), used for both the Public and Auth/Admin web apps
  - Go 1.22+ stdlib router supports method-based routing and path params (e.g. `GET /pages/{id}`), so no external router is needed for most cases
  - Zero external routing dependencies = fewer things to go stale over time, fits the long-term security goal
  - Trade-off: slightly more boilerplate for things like middleware chaining, acceptable for a small 2-service project
- **Postgres driver:** `pgx`, using plain `database/sql`-style access (no heavy ORM) to keep the stack auditable
- **Framework libraries deliberately avoided:** `gin`, `echo`, `fiber`, `beego`, `gorm` — bigger dependency trees and more long-term CVE surface than needed for this project

## Port Assignments

| Port | Protocol | Accessible From | Service |
|------|----------|------------------|---------|
| 8111 | TCP | Localhost only / behind Caddy | Public web app (Go) |
| 5411 | TCP | Internal only (public-net) | Public Postgres |
| 8002 | TCP | Localhost only / SSH tunnel only | Auth/Admin web app (Go) |
| 5402 | TCP | Internal only (admin-net) | Auth Postgres |

## Templating
- **Decision:** use Go's standard library `html/template` package for rendering public pages — no external templating library
- Fits the "avoid dependency rot" principle — part of stdlib, nothing to go stale
- Auto-escapes output by default, which protects against XSS from admin-entered content (page bodies, articles, etc.) being rendered publicly
- Concept: one shared layout template (header/nav/footer) + content-specific templates (page, article, service) that fill in placeholders like `{{.Title}}`, `{{.Body}}`, `{{.Images}}`
- Flow: DB stores raw content → admin app edits it → public app fetches content, passes it into `html/template`, renders final HTML, serves to browser

## Content Management Concept (Admin Module)
Content types the admin module will manage, stored in the public content DB, read-only for the public app:
- **Pages** — static-ish content blocks (About, Services overview, Mission/Vision, etc.) — title, body text, slug for URL
- **Contact info** — phone, email, address, hours — likely a singleton record, always editable, never added/deleted as a list
- **Pictures/media** — images used across pages/articles/gallery — file reference + metadata (caption, alt text, associated page/article)
- **Articles** — blog/news-style posts — title, body, publish date, possibly author, possibly linked images

Relationship concept:
- Pages and Articles can each have associated images (one-to-many)
- Everything admin creates/edits stays in the auth-side flow until "published," then becomes visible via the public app (ties back to the open content flow question below)

**Additional confirmed decisions:**
- **Draft vs published state:** content will have a `status` column (e.g. draft / published) rather than being written directly to "live" — allows preparing and previewing content before making it public
- **Soft-delete:** deletions will use a `deleted_at` timestamp rather than removing rows outright — easier recovery from mistakes than restoring from backup
- **Image storage:** images stored as files on a separate/shared storage volume, with only the path/filename referenced in the DB — not stored as bytes in Postgres
- **Migrations:** schema changes will be handled via a migration tool (`golang-migrate`) run automatically on container startup, rather than manual SQL — important given the admin container isn't always running

*(Schema/table design deferred until concept is fully aligned.)*

## Content Format
- **Decision:** article/page body content stored as **Markdown** (plain `TEXT` column in DB)
- Rationale:
  - Plain text alone is too limited (no formatting, links, inline images)
  - Raw HTML would require marking content as "safe" for `html/template`, which reopens XSS risk — undermines the auto-escaping protection already decided on
  - Markdown is human-readable, diff-friendly, easy to back up, and stable long-term (syntax doesn't rot)
- **Render pipeline:** Markdown (DB) → converted to HTML via `goldmark` (small, actively maintained, low-dependency Markdown library) → passed into `html/template` for final rendering
- **Extra safety step:** if raw HTML is ever allowed inside Markdown content, run the converted output through an HTML sanitizer (e.g. `bluemonday`) before rendering, as a second layer of XSS protection

## Open Questions / Decisions Needed
*(none currently — all prior open items have been resolved; see Content Flow, Auth Design, and Network Segmentation sections)*

## Slugs / URLs
- **Decision:** Hybrid approach (Option C) — slug is auto-generated from the title as a default/suggestion when creating a page/article, but the admin can edit/override it before saving
- **Uniqueness:** enforced at the DB level via a `UNIQUE` constraint on the `slug` column — not just an app-level check, so duplicates can't slip through
- **Collision handling:** on duplicate slug, reject and prompt admin to choose a different one (simpler and more predictable than auto-appending a suffix for a solo-managed site)

## Caddy Configuration Plan
Based on the existing Caddyfile pattern already in use on this server (shared snippets + one site block per domain), the new project will follow the same structure:

- **Domain:** not yet decided — to be purchased later. Once available, add it to:
  - The `@no_host` allowlist in the existing catch-all `:80, :443` block (rejects unrecognized hosts with `444`)
  - A new dedicated site block (see below)
- **New site block (public app), following the existing pattern:**
  ```
  <new-domain> {
      import block_bots
      import block_hackers
      import logging
      header {
          X-Content-Type-Options "nosniff"
          X-Frame-Options "SAMEORIGIN"
          X-XSS-Protection "1; mode=block"
          Referrer-Policy "strict-origin-when-cross-origin"
          -Server
          -X-Powered-By
          -alt-svc
      }
      reverse_proxy public-app:8111
  }
  ```
- **Media/static files:** add a `handle /media/*` block (same pattern as the existing `/docs/*` handler on freeblackfoot.org) to serve directly from the shared volume's live folder, bypassing the Go app for static assets
- **Admin app:** intentionally has **no** Caddy site block / no public routing at all — stays reachable only via SSH tunnel to `127.0.0.1:8002`, entirely outside Caddy's routing, consistent with the "never exposed to the internet" design
- **Existing protections reused as-is:** `block_bots` (IP blocklist) and `block_hackers` (blocks `/wp-admin*`, `/.env*`, `/.git*`, etc.) apply automatically — free protection even though this project has no WordPress/PHP

## Source Control — GitHub Setup on Server

**Repository:** private GitHub repo — `https://github.com/sergiimarchuk/renovaora-com` (private visibility, README + Go `.gitignore` added at creation, no license)

**Additional `.gitignore` entries to add manually** (beyond the default Go template), since they contain secrets or user-uploaded content and must never be committed:
```
.env
*.env
/media/staging/
/media/live/
docker-compose.override.yml
```

**SSH key (deploy key) setup on the server:**
- A dedicated SSH keypair is used for the server to authenticate to GitHub — separate from the key used to log into the server itself (two completely different purposes/keypairs, no overlap)
- Key generated with:
  ```bash
  ssh-keygen -t ed25519 -f ~/.ssh/git_automation_key -C "renovaora-com deploy key"
  ```
  - `ed25519`: modern elliptic-curve algorithm — smaller, faster, and stronger than older RSA keys, current best-practice default for SSH
  - Passphrase set on this key for extra protection (means git operations will prompt for it unless cached via `ssh-agent`/`keychain` — deliberately left non-automated for now, see note below)
- Correct permissions enforced:
  ```bash
  chmod 600 ~/.ssh/git_automation_key       # private key — owner read/write only
  chmod 644 ~/.ssh/git_automation_key.pub   # public key — world-readable is fine
  ```
- SSH config entry (`~/.ssh/config`) to ensure this specific key is used for GitHub:
  ```
  Host github.com
      HostName github.com
      User git
      IdentityFile ~/.ssh/git_automation_key
      IdentitiesOnly yes
  ```
- Public key added to GitHub under **Settings → SSH and GPG keys** (or as a repo-scoped Deploy Key)
- Verified working with:
  ```bash
  ssh -T git@github.com
  ```
  Expected result: `Hi sergiimarchuk/renovaora-com! You've successfully authenticated, but GitHub does not provide shell access.`

**Passphrase caching — deliberately not automated:**
- `ssh-agent` / `keychain` could cache the passphrase across sessions, but for a low-traffic solo project (occasional pushes, not constant active development), typing the passphrase on each push is an acceptable small cost
- Avoids a long-lived decrypted key sitting in memory on a server that also runs other production services
- Can be revisited later if push frequency increases enough to make this annoying

**Important operational note:** the server's own SSH login access (`~/.ssh/authorized_keys`) is completely independent of this deploy key — mixing these up during initial setup caused some confusion, but they serve entirely different purposes (incoming login vs outgoing GitHub auth) and neither affects the other.

**Working directory decision:** the repo is tied to `/opt/web` on the server (not `/home/ubuntu/web.backup/renovaora-com`, which was an earlier candidate path).

**Note:** `/opt/web` contains a leftover `renovaora-com.backup-m1` folder from a prior attempt — this should be excluded from the repo (add `renovaora-com.backup-m1/` to `.gitignore`) or moved out of the directory entirely before the first commit, rather than committed as part of the new project.

**Connecting the local working directory to the GitHub repo:**
```bash
cd /opt/web

# Set git identity (required before any commit)
git config --global user.email "your-email@example.com"
git config --global user.name "Sergii Marchuk"

# Pull the README/.gitignore GitHub already created
git pull origin main --allow-unrelated-histories

# Stage and commit anything already in the directory
git add .
git commit -m "Initial commit"

git branch -M main
git push -u origin main
```

## Content Flow
- **Decision:** single public content DB (not split into separate draft/live databases) — admin and public app share the same content DB
- Draft/publish is handled via the `status` column (draft/published) already decided in Content Management Concept — no data sync/export mechanism needed
- Rationale: strict DB separation is reserved for the real security boundary (auth/credentials), not for draft vs published content; a single DB with a status flag avoids sync bugs, schema-drift, and unnecessary complexity
- **Images/media follow the same staging idea, but physically:**
  - Admin uploads to a staging folder on the shared volume (e.g. `/media/staging/`)
  - On publish, the image is moved/copied to the live folder (e.g. `/media/live/`) that Caddy actually serves from
  - Prevents an unpublished article's images from being reachable via a guessable URL before publish
- The auth DB (users/passwords/sessions) remains fully separate, isolated, and untouched by this — no change to that boundary

## Operational (deferred, flagged for later)
- Backup scripts for both DBs, especially triggered around admin container start/stop
- Audit log table in auth DB (who logged in, what changed, when)
- Documented start/stop steps for whoever has SSH access
- Firewall rules confirmed at deployment time

## Build Order
1. Scaffold folder structure + two Dockerfiles + docker-compose (admin service using a Compose "profile" so it doesn't autostart)
2. Public Go app: static content serving + Postgres read
3. Auth Go app: login, JWT/session, CRUD for content, write path to public DB
4. Wire up networks + volumes with correct isolation
5. Caddy + backups + logging (later, as agreed)

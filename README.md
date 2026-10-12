# Mesh

Mesh is a creator intelligence platform for publishing one content asset across social platforms and understanding how it performs everywhere.

The product will treat the original asset as one content object, with a platform-specific post for each publication. The first version will support connecting accounts, uploading content, publishing or scheduling it, and collecting basic performance data.

Cross-posting is the entry point. Mesh's long-term goal is to help creators learn what content works, where it works best, and what to make next.

## Repository structure

- `apps/web` - Next.js frontend with Clerk sign in
- `apps/api` - Go backend API
- `apps/gateway` - Go reverse proxy in front of the API
- `apps/api/migrations` - Goose PostgreSQL migrations
- `plan` - product brief and the active implementation checklist
- `infra` - placeholder for infrastructure configuration
- `docs` - placeholder for project documentation

## Current status

Development happens in small, runnable steps tracked in [plan/implementation-checklist.md](plan/implementation-checklist.md).

Milestone 0 (runnable authenticated foundation) is complete. Docker Compose runs PostgreSQL, migrations, the API, the gateway, and the web app. Clerk sign in maps to an internal user via `GET /me`. Pull request CI runs Go tests against PostgreSQL, `go vet`, `govulncheck`, `npm audit`, and the web build.

Milestone 1 (a local fake publishing loop) is in progress. Done so far: content metadata in PostgreSQL; authenticated `POST /content` and `GET /content` scoped to the signed in creator; a web page to create and list content; one local media file per content item via authenticated `POST /content/{id}/media` (MP4 or WebM, up to 100 MiB, stored in a Docker volume); and the fake platform account schema with ownership, uniqueness, validation, and migration lifecycle tests (M1.5).

Next: authenticated creation and listing of fake platform accounts (M1.6), followed by platform specific posts, synchronous publishing to a fake platform, and the web screens for upload, account selection, and publishing.

Not yet built: real social platform integrations, metrics, and cloud deployment.

## Running locally

1. Copy `.env.example` to `.env` and set the Clerk development keys (`NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY`, `CLERK_SECRET_KEY`).
2. Run `docker compose up --build`.
3. Open http://localhost:3000.

Only the web service publishes a port to the host (3000).

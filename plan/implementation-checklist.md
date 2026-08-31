# Implementation Checklist

This is the working sequence for small, runnable changes. A completed item
means it is merged into `main`; the detailed product rationale remains in the
[bootstrap plan](creator-platform-codex-bootstrap.md).

## Foundation

- [x] Repository conventions - ignore generated files and secrets, define tab indentation, and document the planned layout. Verify with Git ignore checks and whitespace checks.
- [x] API executable - add a standalone Go module that starts successfully. Verify with `go run .` and `go build`.
- [x] Gateway executable - add a standalone Go module that starts successfully. Verify with `go run .` and `go build`.
- [x] Web starter - add the smallest Next.js and TypeScript application. Verify with `npm run dev`.
- [x] API health endpoint - return `{"status":"ok"}` from `GET /health`. Verify with one Go test and `curl`.
- [x] Gateway health endpoint - return `{"status":"ok"}` from its own `GET /health`. Verify with one Go test and `curl`.
- [x] Gateway API proxy - proxy only `/api/health` to the API. Verify a request through the gateway reaches the API.
- [x] Web health display - show the API health result through the gateway. Verify the browser-to-API path locally.

## Local Product Data

- [x] Local containers - add Docker Compose for the working API, gateway, and web applications. Verify `docker compose up` starts them.
- [x] PostgreSQL connectivity - add a local PostgreSQL container and an API connectivity check. Verify the API reaches the database.
- [x] Migrations - introduce one migration tool and an initial `users` table. Verify applying the migration to a clean local database.
- [ ] Users - create and fetch users through the API. Verify with API tests against the database.
- [ ] Authentication - add hosted authentication and map its identity to an internal user. Verify a protected route rejects anonymous access.

## Content and Fake Publishing

- [ ] Content metadata - persist a user's content record without media upload. Verify create and list operations.
- [ ] Local media upload - attach one media file to content and store it locally. Verify upload validation and persisted metadata.
- [ ] Platform accounts - persist a manually created fake platform account. Verify it belongs to the signed-in user.
- [ ] Posts - persist the relationship between content, a platform account, and its platform-specific post. Verify one content item can have multiple posts.
- [ ] Fake publisher - publish a post to a deterministic fake provider and save its external ID. Verify the full API flow.
- [ ] Publish UI - upload content, choose the fake account, publish, and display the resulting status. Verify the first end-to-end product loop in the browser.

## First Real Platform

- [ ] YouTube account connection - implement OAuth and secure token storage. Verify a creator can connect one YouTube account.
- [ ] YouTube publishing - upload and publish one video, recording platform status. Verify it appears on YouTube.
- [ ] YouTube metrics - fetch and store basic metrics for a published post. Verify a known post is mapped to its metrics.
- [ ] Content dashboard - show a content item with its YouTube publication and metrics. Verify the browser view uses persisted data.

## Earned Complexity

- [ ] Object storage - move media from local disk to S3 only when deployment requires it. Verify existing uploads still work.
- [ ] Asynchronous publishing - add a queue and worker only when synchronous publishing needs reliable retries. Verify a queued publish completes once.
- [ ] Idempotent publishing - prevent retries from producing duplicate external posts. Verify repeated delivery produces one publication.
- [ ] Second platform - add one additional platform before cross-platform comparisons. Verify one content item publishes to both platforms.
- [ ] Unified analytics - compare posts for the same content across platforms. Verify totals and platform-specific metrics agree with stored data.

## Later, Only When Justified

- [ ] Scheduling and retries
- [ ] More platform integrations
- [ ] Creator baselines and recommendations
- [ ] Sponsorship, attribution, and revenue reporting
- [ ] Controlled API or MCP access for authorized agents

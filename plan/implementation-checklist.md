# Implementation Plan

This is the active implementation sequence. The
[bootstrap brief](creator-platform-codex-bootstrap.md) remains product and
architecture context, but its original numbered roadmap is historical.

Each checkbox below is one pull request. Every PR must introduce one primary
behavior, keep `main` runnable, state what is out of scope, and include the
smallest verification that would fail if the behavior broke.

Milestone IDs such as `M1.4` are stable plan references, not predicted GitHub
pull request numbers.

Only the current milestone is committed. Before starting another milestone,
review its purpose, completion gate, assumptions, and PR breakdown with the
developer. Change the plan when completed work teaches us something new.

## Milestone 0: Runnable authenticated foundation - complete

The repository now has the smallest local system on which product behavior can
be built:

- [x] Repository conventions
- [x] Standalone Go API and gateway executables
- [x] Next.js web application
- [x] API and gateway health endpoints
- [x] Gateway reverse proxy and web health display
- [x] Docker Compose for the local system
- [x] PostgreSQL connectivity and Goose migrations
- [x] Internal users persisted in PostgreSQL
- [x] Clerk authentication mapped to internal users through `/me`
- [x] Pull request CI for Go tests, audits, and the web build

Completion evidence: a signed-in browser can reach the web app, traverse the
gateway, authenticate with the API, and map the Clerk identity to PostgreSQL.

## Milestone 1: Complete a local fake-publishing loop - current

### Why this milestone

Mesh's central claim depends on one content object producing platform-specific
posts. Before paying the cost of social APIs or cloud infrastructure, we need to
prove that model through the full browser-to-database path. A deterministic
fake platform gives fast tests and removes OAuth, app review, quota, and outage
risk while the domain model is still changing.

### Completion gate

A signed-in creator can create content, attach one local media file, create and
select a fake platform account, publish synchronously, and see the saved post
status and deterministic external ID in the browser.

### M1.1: Persist content metadata

- [ ] Add a Goose migration for `content` with an owner, title, and timestamps.
- Scope: schema, foreign key to `users`, and migration verification.
- Out of scope: HTTP routes, media fields, posts, and UI.
- Verify: migrate up on a clean database, enforce ownership and required title
  constraints, then migrate down.

### M1.2: Create and list owned content

- [ ] Add authenticated `POST /content` and `GET /content` routes through the
  API, gateway, and web rewrite.
- Scope: non-empty title validation, current-user ownership, stable JSON, and
  user-scoped listing.
- Out of scope: update, delete, pagination, media, and UI.
- Verify: anonymous access is rejected, invalid input is rejected, a created
  record is returned, and one user cannot list another user's content.

### M1.3: Add local media metadata

- [ ] Add nullable media metadata to `content` for one original file.
- Scope: original filename, MIME type, byte size, and local storage key.
- Out of scope: file transfer, multiple assets, thumbnails, and S3.
- Verify: migrate existing content safely and enforce one valid metadata shape
  when a storage key is present.

### M1.4: Upload one local media file

- [ ] Add authenticated `POST /content/{id}/media` for one owned content item.
- Scope: streaming multipart input to a configurable local directory, a fixed
  size limit, an explicit allowlist of supported MIME types, collision-safe
  storage keys, persisted metadata, and a Docker volume for the media directory.
- Out of scope: downloads, resumable upload, transcoding, virus scanning, and
  cloud storage.
- Verify: reject anonymous, oversized, unsupported, and foreign-user uploads;
  save one valid file and its metadata; retain it across an API container
  restart; remove a partially saved file when the database update fails.

### M1.5: Persist platform accounts

- [ ] Add a Goose migration for `platform_accounts` owned by a user.
- Scope: platform, external account ID, display name, timestamps, and a unique
  external identity per platform.
- Out of scope: OAuth credentials, tokens, and HTTP routes.
- Verify: migrate up and down, reject duplicate external identities, and
  enforce user ownership.

### M1.6: Create and list fake platform accounts

- [ ] Add authenticated create and list routes for `fake` platform accounts.
- Scope: current-user ownership and deterministic local account data.
- Out of scope: real providers, OAuth, update, and delete.
- Verify: anonymous access is rejected, duplicate accounts are rejected, and
  users cannot list each other's accounts.

### M1.7: Persist platform-specific posts

- [ ] Add a Goose migration for `posts` linking content to a platform account.
- Scope: owner-safe relationships, caption, status, external post ID, and
  timestamps.
- Out of scope: a public post-creation route, scheduling, metrics, and retries.
- Verify: migrate up and down, allow multiple platform posts for one content
  item, and reject relationships across different users.

### M1.8: Publish synchronously to the fake platform

- [ ] Add authenticated `POST /content/{id}/publish` for one fake account.
- Scope: validate ownership and uploaded media, create one post, return a
  deterministic external ID, and persist `published` status.
- Out of scope: a generic provider interface, background jobs, retries,
  idempotency, scheduling, and real network calls.
- Verify: reject invalid ownership and missing media, then prove one request
  creates the expected saved post and external ID.

### M1.9: Create content and upload media in the web app

- [ ] Add the smallest signed-in form for a title and one media file.
- Scope: create content, upload its file, and show success or a useful error.
- Out of scope: editing, deletion, drag and drop, progress bars, and styling
  beyond accessible native controls.
- Verify: web build passes and a manual browser check creates a database record
  with a saved local file.

### M1.10: Create and select a fake account in the web app

- [ ] Add a small account section that creates and lists fake accounts.
- Scope: accessible native controls and selection of one owned account.
- Out of scope: real provider branding, OAuth, account settings, and deletion.
- Verify: web build passes and a manual browser check creates and selects a
  persisted fake account.

### M1.11: Publish and display the fake result

- [ ] Add a caption and publish action for uploaded content and the selected
  fake account.
- Scope: call the publish route and display saved status and external ID.
- Out of scope: scheduling, retries, optimistic updates, metrics, and a general
  dashboard.
- Verify: web build passes and one browser flow completes the milestone gate.

### Milestone review before M2

Discuss whether `content`, `platform_accounts`, and `posts` express the real
product clearly; whether synchronous publishing is still sufficient; and what
the fake loop revealed before freezing an abstraction for a real provider.

## Milestone 2: Publish one real video to YouTube - proposed

### Why this milestone

The fake loop proves our own system, but not the assumptions imposed by a real
platform. YouTube is the first external constraint: OAuth, token lifecycle,
upload semantics, quotas, and processing states. One provider is enough to
learn those constraints without inventing a premature cross-platform model.

### Completion gate

A signed-in creator can connect one YouTube channel, publish one uploaded video
through Mesh, and open the resulting video on YouTube. Credentials are not
stored in plaintext.

### M2.1: Store encrypted provider credentials

- [ ] Extend platform accounts with encrypted access and refresh credentials
  and expiry metadata.
- Scope: application-managed encryption using a required local key, narrow
  helpers, and schema changes that never return secrets in JSON.
- Out of scope: AWS Secrets Manager, key rotation, and multiple key versions.
- Verify: ciphertext differs from plaintext, decrypts correctly, and malformed
  or wrong-key ciphertext fails closed.

### M2.2: Complete the YouTube OAuth backend flow

- [ ] Add an authenticated start route and provider callback using Google's
  current OAuth requirements.
- Scope: signed single-use state bound to the initiating internal user,
  least-required scopes, channel identity lookup, encrypted token persistence,
  and mocked HTTP tests.
- Out of scope: publishing, metrics, multiple channels, and account deletion.
- Verify: reject invalid state and provider errors, then persist one mocked
  connected channel without exposing tokens.

### M2.3: Connect and display a YouTube channel in the web app

- [ ] Add a YouTube connect action and show the connected channel.
- Scope: redirect through the backend OAuth flow and render connection status.
- Out of scope: publishing controls, disconnect, channel switching, and visual
  polish.
- Verify: web build passes and a manual OAuth check returns to Mesh with the
  connected channel visible.

### M2.4: Introduce the provider publishing boundary

- [ ] Extract only the behavior shared by the working fake publisher and the
  required YouTube publisher.
- Scope: the smallest interface needed to publish existing content and return
  an external ID and status.
- Out of scope: authorize, metrics, scheduling, and a universal platform model.
- Verify: existing fake-publishing tests pass through the new boundary without
  changing observable behavior.

### M2.5: Upload a video with the YouTube client

- [ ] Implement the YouTube publisher against mocked HTTP responses.
- Scope: token refresh when required, upload request construction, provider
  error mapping, and external video ID capture.
- Out of scope: wiring the public route, retries, resumable recovery, metrics,
  and background jobs.
- Verify: mocked success, expired-token refresh, and provider failure paths.

### M2.6: Publish through the existing route to YouTube

- [ ] Select the connected YouTube account in the current publish flow and
  persist its real external ID and status.
- Scope: provider selection, synchronous execution, saved result, and a link to
  the native video.
- Out of scope: processing polling, retries, scheduling, and metrics.
- Verify: automated boundary tests pass and one manual upload appears on the
  connected YouTube channel.

### Milestone review before M3

Discuss upload latency, processing states, quota use, token refresh, failure
handling, and whether synchronous publishing is still the smallest reliable
design. Do not add a queue only because the long-term brief mentions one.

## Milestone 3: Show the outcome of published content - proposed

### Why this milestone

Publishing alone makes Mesh a scheduler. Returning performance data and tying
it to the original content object starts testing the creator-intelligence
thesis. We begin with one provider and observed metrics before designing
cross-platform normalization.

### Completion gate

A creator can return to a published content item and see its current YouTube
views, likes, comments, publication time, and last refresh time.

### M3.1: Persist YouTube metric snapshots

- [ ] Add a migration for timestamped metrics attached to a post.
- Scope: the metrics YouTube actually returns for the published video and a
  collected timestamp.
- Out of scope: cross-platform names, aggregates, retention policy, and UI.
- Verify: migrate up and down and save multiple snapshots for one post.

### M3.2: Refresh metrics for one YouTube post

- [ ] Add an authenticated refresh action that fetches and saves current
  metrics for one owned YouTube post.
- Scope: ownership, token refresh, provider errors, and one saved snapshot.
- Out of scope: scheduled polling, bulk refresh, queues, and webhooks.
- Verify: reject foreign posts and save a mocked provider response exactly once
  per request.

### M3.3: Read content with posts and latest metrics

- [ ] Add an authenticated content-detail response containing its posts and
  each post's latest metric snapshot.
- Scope: one content item, ownership, stable JSON, and latest-snapshot query.
- Out of scope: totals, recommendations, pagination, and cross-platform
  normalization.
- Verify: return the newest snapshot and never expose another user's content.

### M3.4: Display a content performance page

- [ ] Add a page showing content, its YouTube post, latest metrics, and a manual
  refresh action.
- Scope: accessible loading, empty, success, and error states.
- Out of scope: charts, automatic polling, comparisons, and recommendations.
- Verify: web build passes and a manual browser check displays refreshed known
  metrics.

### Milestone review before deployment or a second platform

Discuss whether the data is useful to a creator, which metric definitions need
preserving, and whether the next learning goal is beta deployment or a second
platform. That choice should be based on product need, not roadmap order.

## Conditional milestone: Deploy a beta

Choose this only when another person needs reliable access outside the local
machine. At that point, create a dedicated milestone plan that splits storage,
secrets, database, images, networking, compute, routing, migrations, and
rollback into independently reviewable PRs. Do not create AWS resources before
that trigger exists.

Completion gate: an invited creator can use the proven local behavior in a
recoverable hosted environment without developers handling production secrets
or data manually.

## Conditional milestone: Prove cross-platform comparison

Choose this after the YouTube feedback loop works and a second provider is
selected from current API access and creator demand. Split connection,
publishing, metrics, comparison API, and comparison UI into separate PRs.

Completion gate: one content object maps to posts on two platforms and the
creator can compare clearly defined metrics without losing platform-specific
meaning.

## Conditional milestone: Earn publishing reliability

Choose this only after observed failures show that synchronous requests are not
reliable enough. Plan idempotency before retries, then split queue
infrastructure, worker execution, retry policy, dead-letter handling, and
status UI into separate PRs.

Completion gate: retrying a failed publish cannot create a duplicate external
post, and terminal failures are visible and recoverable.

## Later, only when justified

- Scheduling
- More platform integrations
- Automated metric collection
- Creator baselines and recommendations
- Sponsorship, attribution, and revenue reporting
- Controlled API or MCP access for authorized agents
- Advanced observability
- Video processing and derived media

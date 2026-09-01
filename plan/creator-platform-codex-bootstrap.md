# Creator Platform — Codex Bootstrap Brief

> Working product name: **Mesh**
>
> **Mesh** reflects the long-term vision of a network connecting creators, content, social platforms, brands, and eventually AI agents.
>
> The name is provisional and can be revisited before public launch.
>
> Repository name: `mesh`

---

## 1. What We Are Building

A **creator intelligence platform** that sits between creators and the social platforms they use.

The initial product allows creators to:

- connect their social accounts
- upload one piece of content
- choose where it should be published
- publish or schedule it across supported platforms
- bring performance data back into one place

The longer-term product is **not just another social scheduler**.

The core idea is to treat one underlying video/content asset as a single **content object**, even if it is published to multiple social platforms.

Example:

```text
                    CONTENT #152
                         │
          ┌──────────────┼──────────────┐
          ▼              ▼              ▼
       TikTok        IG Reel       YT Short
        1.4m           620k           310k
          │              │              │
          └──────────────┼──────────────┘
                         ▼
                 CONTENT INTELLIGENCE
```

The system should eventually answer questions such as:

- Which platform performs best for this creator?
- Which content formats consistently outperform?
- Which topics generate views vs followers?
- Which platform produces the most valuable audience growth?
- Which content characteristics correlate with strong performance?
- Which content converts into revenue?
- What should this creator make next?
- Where should they publish it?

The long-term goal is to become a **creator operating system / creator intelligence layer** across social platforms.

---

## 2. The Differentiation

Multi-platform publishing already exists.

Competitors such as:

- Buffer
- Metricool
- Hootsuite
- Later
- Sprout Social
- Repurpose.io
- OpusClip

already provide some combination of:

- multi-platform publishing
- scheduling
- analytics
- AI assistance
- influencer/brand tools
- MCP or agent integrations

Therefore:

> **Cross-posting is an entry feature, not the moat.**

The differentiating thesis is:

> **One platform that learns what makes a creator's content work across every social network.**

Existing social networks understand their own ecosystem.

This product should understand:

```text
Creator
  │
  ├── Content
  │     ├── Platform post
  │     ├── Platform post
  │     └── Platform post
  │
  ├── Audience
  ├── Performance
  ├── Growth
  ├── Sponsorships
  └── Revenue
```

The long-term valuable dataset is:

```text
Content
  → Content characteristics
  → Distribution
  → Platform performance
  → Audience response
  → Conversion
  → Revenue
```

---

## 3. Product Evolution

### V1 — Distribution + Unified Content Model

- authentication
- creator accounts
- social account connections
- upload content
- select platforms
- per-platform captions/settings
- publish
- scheduling
- status tracking
- unified post mapping
- simple analytics ingestion

### V2 — Cross-Platform Intelligence

- content-level analytics
- compare performance across platforms
- creator baselines
- historical performance
- identify repeatable content patterns
- recommendations
- platform-specific optimization

### V3 — Creator Business Layer

- sponsorship campaigns
- attribution
- campaign analytics
- tracked links
- impressions
- clicks
- conversions
- revenue
- brand reporting

### Long-Term

Expose controlled APIs and/or MCP tooling so authorized AI agents can:

- inspect creator performance
- analyze historical content
- prepare platform-specific posts
- recommend distribution
- schedule content
- publish content
- retrieve campaign performance

AI execution must always be permissioned and controlled.

---

# 4. Engineering Goal

This project is intentionally being built as both:

1. a real startup/product
2. a practical systems-engineering education

The project must **not be vibe coded**.

Every pull request should:

- be small enough to understand
- introduce one major concept at a time
- leave the application working
- include tests where appropriate
- have a clear purpose
- be explainable by the developer
- avoid unnecessary abstraction
- avoid infrastructure before there is a problem requiring it

Guiding principle:

> **Do not add technology because "real companies use it". Add it when the problem it solves appears.**

Do not prematurely introduce:

- Kubernetes
- Kafka
- service mesh
- dozens of microservices
- complex event buses
- Redis without a use case
- distributed systems purely for appearance

We want to **earn complexity**.

---

# 5. Chosen Technology Stack

## Frontend

- Next.js
- TypeScript
- React

Potential future additions:

- Playwright
- Vitest

---

## Backend

Language:

- **Go**

HTTP:

- standard `net/http`
- `chi` router

Avoid large Go web frameworks initially.

We want to understand:

- HTTP
- handlers
- middleware
- request lifecycles
- context
- timeouts
- graceful shutdown
- services
- repositories
- interfaces

---

## Database

- PostgreSQL

Go access:

- `pgx`
- `sqlc`

Migrations:

- `goose`

Do not use an ORM unless a clear need appears.

Prefer explicit SQL and generated strongly-typed Go code.

---

## Object Storage

- AWS S3

Used for:

- uploaded videos
- creator media
- eventual generated variants/thumbnails

Never store video binaries inside PostgreSQL.

---

## Infrastructure

- AWS
- Terraform

Terraform should own infrastructure configuration.

Potential AWS components over time:

- VPC
- subnets
- security groups
- ECS
- Fargate
- ECR
- RDS PostgreSQL
- S3
- SQS
- Secrets Manager
- IAM roles/policies
- CloudWatch
- ALB
- Route53
- CloudFront if needed later

---

## Secrets

Use:

- **AWS Secrets Manager**

Sensitive values include:

- OAuth client secrets
- third-party API secrets
- DB credentials where appropriate
- signing secrets
- platform credentials

Rules:

- never commit secrets
- never bake secrets into Docker images
- avoid storing real secret values directly in Terraform state
- Terraform may create secret containers/resources
- values should preferably be populated separately
- ECS tasks should retrieve secrets via IAM-authorized mechanisms

No long-lived AWS access keys in deployed services.

Production workloads should use IAM roles.

---

## Compute

Initial production compute:

- ECS
- Fargate

Meaning:

```text
Go binary
  ↓
Docker image
  ↓
ECR
  ↓
ECS service/task
  ↓
Fargate runs the container
```

Fargate answers:

> Where does this container run?

It is separate from routing concerns.

---

## CI/CD

- GitHub
- Buildkite

Buildkite eventually handles:

- Go tests
- frontend tests
- linting
- Docker builds
- Terraform validation
- Terraform plan
- deployment
- production release flow

Do not overbuild CI in the first PR.

---

## Observability

Use Datadog later.

Do not add it before there is a deployed system worth observing.

Eventually:

- structured logs
- metrics
- traces
- request IDs
- queue metrics
- publishing latency
- API error rates
- platform-specific failures
- DB performance
- alerts

---

# 6. Architecture Direction

Do **not** start with a large microservice architecture.

Initial architecture:

```text
                    Internet
                       │
                       ▼
                 Go API Gateway
                       │
                       ▼
                  Go Backend API
                       │
              ┌────────┼────────┐
              ▼        ▼        ▼
          PostgreSQL   S3    Social APIs
```

Initial Go binaries:

```text
cmd/
  gateway/
  api/
```

The main API should be a modular monolith.

Example internal structure:

```text
internal/
  user/
  content/
  platform/
  publishing/
  analytics/
```

Over time, boundaries may emerge naturally.

For example:

```text
BEFORE

Gateway
   │
   ▼
Monolith
 ├ users
 ├ content
 ├ publishing
 └ analytics
```

If publishing becomes operationally complex:

```text
AFTER

                 Gateway
                /       \
               ▼         ▼
             API      Publishing
              │         Service
         users/content
         analytics
```

Only extract a service when there is a real reason.

---

# 7. Gateway

The developer wants to build a custom gateway in Go for learning purposes.

The gateway is initially a reverse proxy / API gateway, not a true globally-distributed edge network.

Potential routing:

```text
/api/users/*        → API
/api/content/*      → API
/api/analytics/*    → API
/api/publishing/*   → API initially
```

Later:

```text
/api/publishing/*   → publishing-service
```

Future gateway responsibilities may include:

- routing
- authentication middleware
- authorization
- CORS
- request IDs
- structured logging
- request-size limits
- timeouts
- rate limiting
- health checking
- tracing
- API versioning
- service discovery

Do not implement all of these on day one.

---

# 8. Background Jobs

Do not introduce asynchronous infrastructure until synchronous publishing has been built and its limitations are understood.

Eventually use:

- AWS SQS
- Go workers

Publishing should eventually look like:

```text
POST /publish
     │
     ▼
create publishing job
     │
     ▼
SQS
     │
     ▼
Go Worker
     │
     ├── YouTube
     ├── Instagram
     └── TikTok
```

The worker must eventually support:

- retries
- exponential backoff
- idempotency
- failure states
- rate limits
- token refresh
- partial failure
- status updates

Potential states:

```text
QUEUED
UPLOADING
PROCESSING
PUBLISHED
FAILED
```

---

# 9. Core Domain Model

This is extremely important.

## User

```text
users

id
email
display_name
created_at
updated_at
```

---

## Content

A `Content` record represents the creator's original underlying creative asset.

It is **not** a TikTok post.
It is **not** an Instagram Reel.
It is **not** a YouTube Short.

Example:

```text
content

id
creator_id
title
original_filename
storage_key
mime_type
status
created_at
updated_at
```

This is the central domain concept.

---

## Platform Account

Represents an authenticated social account.

```text
platform_accounts

id
user_id
platform
external_account_id
access_token
refresh_token
expires_at
created_at
updated_at
```

Tokens will need stronger security handling in production.

Do not blindly persist OAuth secrets in plaintext forever.

---

## Post

Represents one platform-specific publication of one content object.

```text
posts

id
content_id
platform_account_id
external_post_id
caption
status
scheduled_at
published_at
created_at
updated_at
```

Relationship:

```text
              CONTENT
                 │
       ┌─────────┼─────────┐
       ▼         ▼         ▼
      Post      Post      Post
       │         │         │
    TikTok      IG       YouTube
```

This mapping is core to the business.

---

# 10. Social Platform Abstraction

Do not scatter code like:

```go
if platform == "youtube" {
    ...
}
```

throughout the application.

Create a platform interface.

Conceptually:

```go
type Platform interface {
    Authorize(ctx context.Context, ...) (...)
    RefreshToken(ctx context.Context, ...) (...)
    Publish(ctx context.Context, ...) (...)
    GetPost(ctx context.Context, ...) (...)
    GetMetrics(ctx context.Context, ...) (...)
}
```

Exact method signatures should be designed when required.

Important:

- keep platform-specific code isolated
- make implementations swappable
- enable fake/test implementations
- normalize data carefully
- do not over-generalize platform differences away

---

# 11. Fake Platform

Before integrating a real social API, implement a fake provider.

Example:

```text
FakePlatform
```

Publishing may return:

```text
fake_post_123
```

This allows us to build the complete publishing flow without being blocked by:

- Google OAuth
- Meta review
- TikTok app review
- API quotas
- external outages

The fake platform should support:

- connecting a fake account
- publishing
- generated external IDs
- fake metrics

This is valuable for:

- tests
- local development
- architecture validation

---

# 12. Social API Feasibility Summary

The core MVP is technically feasible using official APIs, subject to each platform's restrictions and approval processes.

## YouTube

Possible:

- OAuth
- video uploads
- analytics
- channel analytics
- scheduling through our own backend logic

Important:

- unverified API projects may face upload visibility restrictions
- quotas and audits apply

Docs:

- https://developers.google.com/youtube/v3/docs/videos/insert
- https://developers.google.com/youtube/analytics

---

## Instagram

Possible for professional accounts:

- OAuth / account connection
- image/video/Reels publishing
- media insights
- account insights

Important:

- primarily Business/Creator accounts
- Meta permissions/app review
- platform-specific publishing restrictions

Docs:

- https://developers.facebook.com/docs/instagram-platform/
- https://developers.facebook.com/docs/instagram-platform/content-publishing/
- https://developers.facebook.com/docs/instagram-platform/instagram-api-with-instagram-login/insights/

---

## TikTok

Possible:

- OAuth
- Content Posting API
- Direct Post
- draft upload
- basic video metrics

Important:

- `video.publish` requires approval/audit
- stricter UX/policy requirements
- platform review risk is meaningful

Docs:

- https://developers.tiktok.com/products/content-posting-api/
- https://developers.tiktok.com/doc/content-posting-api-get-started/
- https://developers.tiktok.com/doc/content-posting-api-reference-direct-post/

---

## Facebook

Possible:

- Facebook Page workflows
- Reels/video publishing
- Page analytics

Important:

- focus on Page/creator/business workflows
- do not design V1 around arbitrary personal profiles

Docs:

- https://developers.facebook.com/docs/video-api/guides/reels-publishing/

---

## X

Possible:

- OAuth
- post creation
- media upload
- engagement metrics

Important:

- API pricing/access may affect commercial viability
- metrics availability varies

Docs:

- https://developer.x.com/en/docs/x-api/posts/manage-tweets/api-reference/post-tweets
- https://developer.x.com/en/docs/x-api/v1/media/upload-media/overview

---

## LinkedIn

Possible:

- OAuth
- posts
- videos
- organization analytics
- sponsored content integrations

Important:

- permissions/product approval vary
- less important for creator-first V1

Docs:

- https://learn.microsoft.com/linkedin/marketing/community-management/shares/posts-api
- https://learn.microsoft.com/linkedin/marketing/community-management/shares/videos-api

---

# 13. First User Experience

The first major demo milestone should be:

```text
1. Open product
2. Sign in
3. Connect one real social account
4. Upload video
5. Select social account
6. Add caption
7. Click publish
8. Open the native platform
9. See the content appear
10. Return later
11. See performance data in our dashboard
```

This is the first true product "wow" moment.

---

# 14. PR Philosophy

Every PR must answer:

1. What problem are we solving?
2. Why are we solving it now?
3. What concept does this PR teach?
4. What changes?
5. What does not change?
6. How do we test it?
7. What new failure modes exist?

Avoid giant PRs.

Prefer roughly:

- one concept
- one domain change
- one infrastructure change

per PR.

---

# 15. Initial PR Roadmap

> Historical note: this was the bootstrap sequence used to establish the
> repository. It is not the active implementation plan and its PR numbers do
> not correspond to current GitHub pull requests. Continue from the
> [implementation plan](implementation-checklist.md).

## PR #1 — Repository Skeleton

Create:

```text
/
├── apps/
│   ├── web/
│   ├── api/
│   └── gateway/
├── infra/
├── docs/
├── .gitignore
├── .editorconfig
└── README.md
```

Goal:

- establish repo structure
- no product functionality

Learn:

- monorepo organization
- frontend/backend separation
- Go module setup
- Node project setup

Done when:

- Next.js app starts
- Go API starts
- Go gateway starts

Do **not** add Docker, database, auth, Terraform, CI, S3, or cloud resources yet.

---

## PR #2 — Go API Health Endpoint

Implement:

```http
GET /health
```

Response:

```json
{
  "status": "ok"
}
```

Use:

- `net/http`
- `chi`

Add tests.

Learn:

- HTTP routing
- handlers
- JSON
- status codes
- Go HTTP testing

---

## PR #3 — Gateway Health + Proxy

Gateway exposes:

```http
GET /health
```

Then proxy a route such as:

```text
/api/* → backend API
```

Keep routing simple.

Learn:

- reverse proxy concepts
- Go `httputil.ReverseProxy`
- upstreams
- headers
- failure handling

Do not add auth/rate limiting yet.

---

## PR #4 — Frontend Shell

Create basic Next.js routes:

```text
/
 /login
 /dashboard
 /settings
```

Dashboard can show:

```text
Mesh

0 connected platforms
0 content items
```

No real auth.

Learn:

- Next.js routing
- layouts
- TypeScript
- component boundaries

---

## PR #5 — Docker

Dockerize:

- API
- gateway
- frontend

Introduce:

```text
docker-compose.yml
```

Do not add production AWS yet.

Learn:

- Docker images
- containers
- ports
- networks
- environment variables
- multi-stage builds if appropriate

Done when:

```bash
docker compose up
```

starts the system.

---

## PR #6 — Local PostgreSQL

Add PostgreSQL to Docker Compose.

Do not create complex schema yet.

Add DB connectivity and health verification.

Learn:

- networked services
- DB connection strings
- connection pools
- failure handling

---

## PR #7 — Migrations

Add:

- `goose`

Create initial migration for:

```text
users
```

Learn:

- schema versioning
- forward migrations
- rollback
- migration discipline

Do not use auto schema creation.

---

## PR #8 — SQLC + User Repository

Add:

- `pgx`
- `sqlc`

Implement user reads/writes.

Potential endpoints:

```text
POST /users
GET /users/{id}
```

Learn:

- SQL
- generated Go types
- repositories
- service boundaries

---

## PR #9 — Authentication

Choose:

- Clerk
- or Auth0

Do not build password management from scratch.

Implement:

- login
- logout
- protected dashboard
- backend identity validation
- mapping external identity → internal user

Learn:

- authentication vs authorization
- sessions/tokens
- middleware

---

## PR #10 — Content Domain

Create `content` migration and Go types.

No file upload yet.

Learn:

- domain modeling
- ownership relationships
- why `Content` differs from `Post`

---

## PR #11 — Local File Upload

Implement:

```http
POST /content
```

Store uploaded media locally.

Persist metadata in PostgreSQL.

Learn:

- multipart uploads
- streams
- content validation
- metadata

---

## PR #12 — Terraform Bootstrap

Introduce Terraform.

Start very small.

Create:

- provider configuration
- dev environment structure
- one S3 bucket

Potential layout:

```text
infra/
  terraform/
    environments/
      dev/
    modules/
```

Learn:

- providers
- state
- resources
- plan/apply/destroy
- remote state later

Do not create entire AWS production architecture in one PR.

---

## PR #13 — S3 Storage

Replace local file storage with an interface:

```go
type ObjectStore interface {
    Put(...)
    Get(...)
    Delete(...)
}
```

Implement:

- local store
- S3 store

Goal:

same product behavior, different storage backend.

Learn:

- AWS SDK for Go
- IAM
- S3
- abstraction boundaries

---

## PR #14 — AWS Secrets Manager

Terraform creates the secret resources/containers.

Do not hardcode secret values into Terraform if it causes them to be persisted in state.

Add a Go secrets abstraction.

Learn:

- secret lifecycle
- AWS SDK
- IAM policies
- environment-specific configuration

---

## PR #15 — Platform Domain

Create:

```text
platform_accounts
```

Define supported platform enum/type.

No real OAuth yet.

Learn:

- external identities
- platform ownership
- database modeling

---

## PR #16 — Platform Interface

Introduce platform abstraction.

Do not integrate YouTube yet.

Learn:

- interfaces
- dependency inversion
- testability

---

## PR #17 — FakePlatform

Implement fake account connection and publishing.

Example:

```text
external_post_id = fake_post_123
```

Learn:

- fake implementations
- deterministic tests
- external system isolation

---

## PR #18 — Post Domain

Create:

```text
posts
```

Connect:

```text
content
→ post
→ platform account
```

Learn:

- relational modeling
- one-to-many relationships
- platform-specific state

---

## PR #19 — Publish Through FakePlatform

End-to-end fake publishing:

```text
Upload content
→ choose fake platform
→ caption
→ publish
→ post row created
→ external fake ID returned
```

This is the first full conceptual product loop.

---

## PR #20 — Terraform Networking Basics

Only now start expanding AWS infrastructure.

Create:

- VPC
- public/private subnets as justified
- routing
- security groups

Learn:

- CIDR
- subnets
- NAT costs/tradeoffs
- public vs private workloads

Keep this PR educational and documented.

---

## PR #21 — ECR

Terraform:

- ECR repositories

Build/push:

- gateway image
- API image

Learn:

- container registry
- image tags
- immutable artifacts

---

## PR #22 — ECS + Fargate Dev Deployment

Deploy:

- API
- gateway

Do not deploy every future component.

Learn:

- ECS task definitions
- services
- task roles
- execution roles
- Fargate
- container health

---

## PR #23 — ALB

Introduce Application Load Balancer in front of gateway.

Flow:

```text
Internet
→ ALB
→ Gateway
→ API
```

Learn:

- target groups
- health checks
- security groups
- load balancers

Only gateway should be internet-facing.

---

## PR #24 — RDS PostgreSQL

Terraform:

- RDS Postgres

Move dev cloud deployment to RDS.

Learn:

- subnet groups
- security
- backups
- DB credentials
- connectivity

---

## PR #25 — First Real Social Integration

Recommended first:

- YouTube

Implement:

- OAuth
- account connection
- token storage
- video upload
- publish status

Do not integrate 5 platforms at once.

Learn one platform deeply.

---

## PR #26 — First Real Analytics Pull

For YouTube:

- retrieve metrics
- map external video → internal post
- persist normalized metrics

Learn:

- polling
- external identifiers
- data normalization
- API quotas

---

## PR #27 — Unified Content Dashboard

For one content object show:

```text
Content #123

YouTube
Views:
Likes:
Comments:
Published:
```

After second platform exists:

```text
Content #123

YouTube      ...
TikTok       ...

Total views ...
```

This begins to demonstrate the actual differentiation.

---

## PR #28 — SQS

Only introduce now, after synchronous publishing exists.

Create queue with Terraform.

Learn:

- message queues
- delivery semantics
- visibility timeout
- dead-letter queues

---

## PR #29 — Go Publishing Worker

Add:

```text
cmd/worker/
```

Flow:

```text
API
→ SQS
→ Worker
→ platform
```

Learn:

- background work
- asynchronous architecture
- graceful shutdown
- long polling

---

## PR #30 — Idempotent Publishing

Prevent duplicate publishes from retries.

Learn:

- idempotency keys
- at-least-once delivery
- duplicate messages
- state transitions

This is a very important distributed-systems PR.

---

# 16. Later PR Ideas

Do not implement these until justified.

- TikTok integration
- Instagram integration
- Facebook Pages
- LinkedIn
- X
- scheduling
- token refresh worker
- webhooks
- retry strategies
- rate limiting
- dead-letter queues
- content processing
- thumbnails
- video transcoding
- analytics snapshots
- content baselines
- content feature extraction
- AI recommendations
- sponsorship campaigns
- tracked redirect links
- conversion attribution
- revenue tracking
- MCP server
- public API
- Datadog
- OpenTelemetry
- Kubernetes

---

# 17. Coding Rules for Codex

When implementing:

1. Do not implement multiple PRs at once.
2. Do not jump ahead in the roadmap unless explicitly asked.
3. Before changing code, explain:
   - what this PR is doing
   - what files will change
   - what concept is being introduced
4. Prefer standard library Go where practical.
5. Do not add large frameworks without justification.
6. Keep dependencies minimal.
7. Write idiomatic Go.
8. Use `context.Context` appropriately.
9. Add timeouts for network operations.
10. Handle errors explicitly.
11. Do not swallow errors.
12. Avoid global mutable state.
13. Prefer dependency injection through constructors.
14. Do not over-engineer interfaces.
15. Only introduce interfaces where substitution/testing/boundaries justify them.
16. Use explicit SQL.
17. Add tests with each meaningful behavior.
18. Keep each PR small.
19. Do not create speculative abstractions.
20. Do not introduce microservices unless explicitly requested.
21. Do not introduce Kubernetes unless explicitly requested.
22. Never put secrets in source code.
23. Never commit `.env` files containing real secrets.
24. Never create long-lived AWS credentials for deployed services.
25. Prefer IAM roles.
26. Terraform should be readable and educational.
27. Avoid giant generic Terraform modules at the beginning.
28. Every infrastructure resource should have a reason to exist.
29. Explain tradeoffs when choices are not obvious.
30. The developer must be able to explain every line merged.

---

# 18. PR Output Format for Codex

For every PR, respond using this structure before implementation:

## Goal

What this PR achieves.

## Why Now

Why this belongs at this point in the architecture.

## Concepts

What the developer should learn from this PR.

## Design

How the implementation will work.

## Files

Expected files created/changed.

## Out of Scope

Things intentionally not included.

## Implementation

Then make the changes.

## Verification

Provide exact commands to verify locally.

Example:

```bash
go test ./...
go run ./cmd/api
curl localhost:8080/health
```

## What You Should Understand Before Merging

Give 3–8 questions the developer should be able to answer.

Example:

- What happens when an HTTP request reaches the Go server?
- Why are we using `chi` instead of a large framework?
- What is the difference between `http.Handler` and `http.HandlerFunc`?
- Why should a health endpoint be simple?

---

# 19. First Codex Task

> Historical note: this task is complete. Use the
> [implementation plan](implementation-checklist.md) for current work.

Start with **PR #1 only**.

Do not implement PR #2 yet.

## PR #1 Scope

Create the monorepo skeleton:

```text
/
├── apps/
│   ├── web/
│   ├── api/
│   └── gateway/
├── infra/
├── docs/
├── .gitignore
├── .editorconfig
└── README.md
```

Requirements:

### `apps/web`

- minimal Next.js + TypeScript app
- must run locally
- no auth
- no dashboard implementation beyond minimal starter page

### `apps/api`

- Go module
- minimal executable
- no HTTP endpoint yet if it is not necessary for PR #1
- must compile

### `apps/gateway`

- separate Go executable/module or appropriately structured Go package
- minimal executable
- must compile
- no proxy behavior yet

### `infra`

- placeholder README only
- no Terraform resources yet

### `docs`

- placeholder architecture/product docs if useful

### Root README

Explain:

- product concept
- repo structure
- local prerequisites
- how to start each application
- that this repository is intentionally developed incrementally

Do not add:

- Docker
- PostgreSQL
- Terraform resources
- AWS SDK
- Secrets Manager
- Buildkite
- Datadog
- authentication
- social APIs
- health endpoints
- reverse proxy
- queues

After implementation:

- show the final file tree
- show commands to run web/API/gateway
- show commands to verify Go builds
- explain every dependency introduced
- provide the questions I should be able to answer before merging

---

# 20. Product One-Liner

Current best internal description:

> **A creator intelligence platform that learns what makes your content work across every social network.**

MVP explanation:

> Creators connect their social accounts, upload content once, distribute it across platforms, and see its combined performance in one place.

The MVP explanation is **not** the differentiation.

The differentiation is cross-platform, content-level creator intelligence.

---

# 21. Mindset

Build slowly.

Prefer:

```text
problem
→ simple solution
→ experience limitation
→ understand limitation
→ introduce next system
```

over:

```text
read architecture blog
→ copy architecture
→ hope it becomes necessary
```

The goal is that at any point in the project, if someone asks:

> "Why does this component exist?"

the developer can answer clearly.

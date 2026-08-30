# Mesh

Mesh is a creator intelligence platform for publishing one content asset across social platforms and understanding how it performs everywhere.

The product will treat the original asset as one content object, with a platform-specific post for each publication. The first version will support connecting accounts, uploading content, publishing or scheduling it, and collecting basic performance data.

Cross-posting is the entry point. Mesh's long-term goal is to help creators learn what content works, where it works best, and what to make next.

## Planned repository structure

- `apps/web` - Next.js frontend
- `apps/api` - Go backend API
- `apps/gateway` - Go reverse proxy
- `infra` - infrastructure configuration
- `docs` - project documentation

## Current status

Repository conventions and product documentation only. No application services or infrastructure have been implemented yet.

The project is intentionally developed in small, runnable steps. The next change will add the first Go executable; it will not add application behavior, infrastructure, or external services.

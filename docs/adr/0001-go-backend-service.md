# Use a Go backend service for Angel

## Context
Angel is a lightweight web application serving daily notes to visiting caregivers and providing an authoring interface for the client. We considered full-stack serverless web frameworks (such as Next.js or Astro) versus a standalone Go daemon/service.

## Decision
We will build Angel as a Go backend service (scaffolded from or aligned with the user's Go `template-service` architecture). The service will serve the frontend and handle API requests for PIN verification, notes, and acknowledgements.

## Reasons
- **Deployment simplicity**: Compiles down to a single binary with zero external runtime dependencies, easy to run on local hardware or any Linux host.
- **Ecosystem consistency**: Matches existing development and service patterns used across the user's Go repositories.
- **Minimal overhead**: High performance and small memory footprint suitable for continuous in-home or lightweight server operations.

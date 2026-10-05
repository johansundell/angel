# Embedded HTML and Vanilla Frontend

## Context
We need a responsive, mobile-first user interface for caregivers entering a PIN and viewing daily notes, as well as an authoring interface for the client. We considered bundling a modern JavaScript SPA framework (React, Vue, Vite) versus embedding standard Go HTML templates and vanilla CSS/JS.

## Decision
The frontend will be built using standard Go `html/template` templates with vanilla HTML, CSS, and minimal JavaScript, embedded directly into the Go binary using `embed.FS`. The user-facing interface text will be in Swedish.

## Reasons
- **Zero build tooling**: Requires no Node.js runtime, npm dependencies, or separate bundling pipeline.
- **Single-binary distribution**: The complete web interface ships inside the Go executable.
- **Fast and lightweight**: Delivers near-instant initial render times on mobile devices over cellular connections.

# Prompt to give the coding agent (copy/paste)

Work in https://github.com/jsoprych/elko-market-mcp

FIRST: read *only* the `todo-now/` directory, then implement the Docker publishing pipeline.

Priority:
1) Add workflow `.github/workflows/docker-image.yml` (from todo-now/20_workflow_docker-image.yml)
2) Update Dockerfile for buildx multi-arch + ensure dashboard assets exist in the final image if required
3) Choose least-disruptive compose change (todo-now/50_compose_delta.md)
4) Only AFTER pipeline works: update existing docs/README organically (minimal)

Rules:
- Do not rewrite README images/layout unless necessary.
- Keep changes minimal and tied to DoD.
- Report: list files changed, then show final outputs.

# Compose delta (choose low-disruption)

Option A (least disruptive):
- KEEP existing docker-compose.yml for dev builds (build: .)
- ADD new file: docker-compose.run.yml (published image)
- Docs: `docker compose -f docker-compose.run.yml up -d`

Option B:
- CHANGE docker-compose.yml to run-only (published image)
- ADD docker-compose.dev.yml for build-from-source

Pick A if you want minimal change to existing dev workflow.
Either way:
- include SEC_USER_AGENT
- persist /data volume

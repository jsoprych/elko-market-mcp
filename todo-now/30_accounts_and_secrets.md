# Accounts / Secrets checklist (minimum)

Required:
1) Docker Hub account (to publish mirror image).
   - Create a Docker Hub Personal Access Token (PAT).
   - Add GitHub repo secrets:
     - DOCKERHUB_USERNAME
     - DOCKERHUB_TOKEN

Covered by GitHub:
2) GHCR (GitHub Container Registry).
   - Uses GitHub identity + GITHUB_TOKEN (packages:write).

Optional later:
- Docker MCP registry submission
- Image signing / vuln scanning

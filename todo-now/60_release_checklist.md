# Release checklist (you)

1) Create Docker Hub PAT; set GitHub secrets:
   - DOCKERHUB_USERNAME
   - DOCKERHUB_TOKEN

2) Merge pipeline work to main.

3) Tag and push:
   - git tag v0.1.1
   - git push origin v0.1.1

4) Verify:
   - docker pull ghcr.io/jsoprych/elko-market-mcp:v0.1.1
   - docker pull jsoprych/elko-market-mcp:v0.1.1

5) Smoke:
   - docker run --rm -p 8080:8080 -e SEC_USER_AGENT="Name email" ghcr.io/jsoprych/elko-market-mcp:v0.1.1
   - open http://localhost:8080
   - curl http://localhost:8080/health

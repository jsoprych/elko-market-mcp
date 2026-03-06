# todo-now/ — Docker publishing sprint (ADD-ON folder)

Purpose: a **single place** for a coding agent to focus on the Docker publishing pipeline work,
WITHOUT rewriting existing repo docs or core assets up front.

Workflow:
1) Agent reads **this folder first** (in order).
2) Agent implements the pipeline changes in the real repo paths (workflow, Dockerfile, compose, etc.).
3) Agent then updates existing docs/README *organically* as part of its normal cycle.

Order to read:
1) 10_DOD.md
2) 20_workflow_docker-image.yml
3) 30_accounts_and_secrets.md
4) 40_dockerfile_delta.md
5) 50_compose_delta.md
6) 60_release_checklist.md
7) 70_agent_prompt.md
8) 80_local_pipeline.md

Key rule: **Do not touch README/docs until the pipeline is working end-to-end.**

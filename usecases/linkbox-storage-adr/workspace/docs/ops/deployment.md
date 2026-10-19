# How linkbox runs

Updated 2026-07-14, when linkbox moved from the team's VM to the company's container platform.

- linkbox runs on the container platform in the `eu-west` region. Platform policy for every HTTP service is at
  least two replicas, spread across zones, behind the platform load balancer; a service cannot opt out of it.
- The load balancer sends each request to any healthy replica, round robin. It has no session affinity.
- Deploys are rolling: the platform starts the new replicas before it stops the old ones, so up to four instances
  of linkbox run at once during a deploy. We deploy two or three times a week, and the platform also moves
  replicas between nodes whenever it needs to.
- A container's filesystem is ephemeral: every start begins from the image, and whatever a container wrote is
  gone when it stops. The platform offers no persistent volumes to application workloads; state belongs in one of
  the platform's data services (`platform-services.md`).
- The platform calls `GET /healthz` every 10 seconds and restarts a replica after three failed checks in a row.
- Configuration comes from environment variables and mounted secrets; the API keys file is a mounted secret.
- The platform's build image has no C toolchain, so binaries are built with `CGO_ENABLED=0`.

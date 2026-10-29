# Docker image publishing

The GitHub Actions publish workflows build and push multi-platform images to
Docker Hub and GHCR after verification succeeds. Pushes to `main` publish the
`latest`, `main`, and commit SHA tags. Version tags such as `v1.2.3` publish
`1.2.3`, `1.2`, and the commit SHA tags. The publish workflow can also be run
from the Actions tab.

Configure these Actions secrets in each repository, or create organization
secrets shared with the participating repositories:

- `DOCKERHUB_USERNAME`: Docker Hub account name with push access to the
  `netgoat-xyz` namespace.
- `DOCKERHUB_TOKEN`: Docker Hub access token with read/write permissions.

Images produced by this deployment are:

| Repository | Docker Hub image |
| --- | --- |
| `netgoat-xyz/netgoat` | `netgoat-xyz/netgoat` |
| `netgoat-xyz/streamer` | `netgoat-xyz/streamer` |
| `netgoat-xyz/cass-admin` | `netgoat-xyz/cass-admin` |
| `netgoat-xyz/telemetry` | `netgoat-xyz/telemetry` |
| `netgoat-xyz/cass` | `netgoat-xyz/cass` |
| `netgoat-xyz/frontend` | `netgoat-xyz/frontend` |

The stream service repository was renamed from `stream-server` to `streamer`.
Its Docker Hub and GHCR image names are both `streamer`.

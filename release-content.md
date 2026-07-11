# NetGoat Agent Release

## Highlights

- Self-hostable reverse proxy agent for NetGoat deployments.
- Built with the current repository source and tested before release packaging.
- Includes a ready-to-edit `config.yml`, project license, README, optional AI helper scripts, and public error-page assets.
- Release archives include SHA-256 checksum files for every packaged artifact.

## Install

1. Download the archive for your operating system from the release assets.
2. Extract it on the host that will run the agent.
3. Edit `config.yml` for your routes, API stream URL, TLS, WAF, cache, metrics, and traffic controls.
4. Run `netgoat-agent` from the extracted folder.

## Upgrade Notes

- Back up your existing `config.yml` and `database/` directory before replacing the binary.
- Keep your `API_STREAM_KEY`, `DiamondKey`, and TLS private keys out of release archives and shell history.
- If Koda WAF or Koda-2 is enabled, confirm the referenced model and Python script paths still match your deployment.

## Verification

Each archive has a matching `.sha256` file. Verify an asset before installing it:

```bash
sha256sum -c netgoat-agent-linux-amd64.tar.gz.sha256
```

On macOS:

```bash
shasum -a 256 -c netgoat-agent-darwin-amd64.tar.gz.sha256
```

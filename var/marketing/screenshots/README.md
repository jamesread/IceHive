# Marketing screenshots

The PNGs in this directory are used by the top-level `README.md` and the docs
(`docs/content/assets/marketing/`). They are captured against a throwaway stack
seeded with fictional data, so they can be regenerated at any time:

```bash
make
```

This builds the controller, `collector-testdata`, `persister-mysql` and the
frontend; starts MySQL and RabbitMQ in containers; seeds config and collection
sources from `seed/`; captures every section of `screenshots.ini` with
`repo-helper screenshot`; copies the docs images; and tears everything down.

Requirements: Go, Node.js, `jq`, `curl`, `podman` (or `make CONTAINER=docker`),
and `repo-helper` with Chrome/Chromium.

| File | Purpose |
|------|---------|
| `screenshots.ini` | Pages to capture (one section per PNG) |
| `scripts/prepare.py` | Waits for the SPA to render data before capture |
| `scripts/light.py`, `scripts/dark.py` | Pin the picocrank theme for a capture |
| `seed/run-history.sql` | Fictional last-run timestamps so sources look healthy |
| `seed/config.json` | Config vars set via `SetConfig` |
| `seed/sources.json` | Fictional collection sources set via `UpsertCollectionSource` |
| `*.yaml`, `mysql-init.sql` | Configs for the throwaway services |

Use `make up seed` to start the seeded stack and browse it at
<http://127.0.0.1:18400/>, then `make down` when finished.

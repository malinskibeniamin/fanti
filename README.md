# Fanti 繁体 · 玉簡閣

Chinese language-learning app for readers moving between scripts: convert real books and
subtitles between Simplified and Traditional Chinese, read them with tap-to-define and ruby
pinyin, and study characters with spaced repetition, quizzes, and handwriting practice.

The **Imperial Archive** visual language uses parchment, lacquer red, antique gold, jade,
and ink throughout the application.

The project-owned brand mark is maintained as the original vector
[`web/public/fanti-mark.svg`](web/public/fanti-mark.svg) and released under the MIT License
with the rest of Fanti.

## Layout

| Path | What |
|---|---|
| `proto/` | fanti.v1 protobuf contracts (buf, AIP resource design, ConnectRPC) |
| `backend/` | Go API server — conversion engine, dictionary, SRS study service |
| `web/` | React SPA — Rsbuild, React 19, TanStack Router, Tailwind 4 |

## Development

```sh
cd web && bun install --frozen-lockfile && cd ..
task dev:db        # Postgres via docker compose
task dev:backend   # Go API server
task dev:web       # Rsbuild dev server
task lint          # golangci-lint + biome + buf lint
task test          # all tests
task db:reset      # reset the dev DB to the seed baseline (wipes progress)
```

Browser verification and the e2e suite mutate the shared dev database; `task db:reset`
returns it to a clean seed baseline in one command. Real study progress should be saved
with `task backup` (and restored with `task restore`) before resetting.

## Back up and restore

Fanti keeps imported books, reading progress, and study history in Postgres. Create a
validated custom-format archive before upgrades or database maintenance:

```sh
task backup
```

The command writes an atomically completed `.dump` file under `backups/` and prints its
path. An incomplete or invalid dump is deleted rather than left looking usable.

Restore one of those archives with:

```sh
task restore -- backups/fanti-<timestamp>-<id>.dump
```

Restore validates the archive before changing the database, requires typing `RESTORE`,
and saves a second `-pre-restore.dump` recovery archive. The Compose app is paused during
the database replacement and restarted afterward. If the requested restore fails, Fanti
automatically restores the recovery archive and exits with an error. Keep the reported
recovery archive until the restored data has been checked.

## Local Docker

```sh
docker compose up --build
```

The app migrates and fully seeds a fresh database before serving traffic. A completion
marker makes later restarts skip the bootstrap without rewriting user data.

> [!WARNING]
> Fanti is currently a single-user, local-first app. Its API has no authentication. The
> default Compose ports bind to localhost; do not expose the container or API to the
> internet until authentication and user isolation are implemented. See
> [SECURITY.md](SECURITY.md).

## Database admin — Querylane (dogfood)

Fanti's Postgres is managed with [Querylane](https://github.com/querylane/querylane)
during development. This is optional. Clone Querylane beside this repository, then start
the admin profile with `docker compose --profile admin up --build`. Register this server:

```
Host: localhost   Port: 5433   Database: fanti   User: fanti   Password: fanti
```

The backend also follows querylane's engineering conventions (golangci config,
connectrpc interceptors, kong/koanf, goose + embedded-postgres tests, Taskfile).

## Licences & data credits

See [NOTICES.md](NOTICES.md) — CC-CEDICT (CC BY-SA 4.0), Hanzi Writer stroke data
(Arphic Public License), OpenCC dictionaries (Apache-2.0).

Fanti's original source code is licensed under the [MIT License](LICENSE). Third-party
datasets and dependencies remain under their respective licences as documented in
[NOTICES.md](NOTICES.md).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for setup and verification instructions. Before
preparing a public release, follow [the public-release checklist](docs/public-release.md).

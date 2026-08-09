# Contributing to Fanti

## Before you start

- Search existing issues before opening a new one.
- Keep each change focused on one behavior or maintenance goal.
- Report security problems privately as described in [SECURITY.md](SECURITY.md).

## Local setup

Install Go, Bun, Buf, Docker, [Task](https://taskfile.dev/), and golangci-lint. Then run:

```sh
cd web && bun install --frozen-lockfile && cd ..
task dev:db
task dev:backend
task dev:web
```

The local app is available at `http://localhost:3000`. Local database credentials in
`README.md` and `docker-compose.yml` are development-only values.

## Make a change

Write a failing test before changing behavior. Keep unit and integration tests beside the
code they cover. Use the repository's generated protobuf files as outputs, not as hand-edited
source.

Before opening a pull request, run:

```sh
task fmt
task lint
task test
docker compose config --quiet
```

Use commit messages in the form `type(scope): description`, where `type` is one of `feat`,
`fix`, `refactor`, `style`, `test`, `docs`, `chore`, `perf`, `ci`, `build`, or `revert`.

## Data and privacy

- Never commit uploaded books, subtitles, database dumps, backups, environment files,
  credentials, or production data.
- Add only datasets and media that permit redistribution. Record the source, version,
  checksum, licence, and required attribution in [NOTICES.md](NOTICES.md).
- Keep third-party data under its original licence. Fanti's MIT licence does not
  replace those terms.
- Use synthetic or public-domain content in tests, screenshots, traces, and examples.

## Contribution licence

Fanti's original source is licensed under the MIT License. By contributing, you confirm
that you have the right to submit the work and agree to license the contribution under the
same terms.

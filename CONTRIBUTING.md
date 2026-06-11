# Contributing to GameNest

Thanks for your interest in GameNest! This is a community project and
contributions — bug reports, fixes, new game templates, docs, features — are very
welcome.

## Quick start

GameNest is a Go **engine** + a React/Vite/Tailwind **UI**, packaged as a Tauri
desktop app. Each game server runs in its own Docker container.

```bash
# Engine: run the test suite (14 packages)
go -C engine test ./...
go -C engine vet ./...

# UI: type-check + build
npm --prefix ui install   # first run only
npm --prefix ui run build
```

See [`README.md`](README.md) for running the app and
[`docs/architecture.md`](docs/architecture.md) for how the pieces fit together.

## Ground rules

- **Tests first.** The engine is developed test-first; please add or update tests
  with behavior changes and keep `go -C engine test ./...` green.
- **Keep it focused.** One logical change per pull request.
- **Match the surrounding style.** Small, clear units; comments where intent isn't
  obvious.
- **New game templates** go in `templates/` as a single YAML file — copy an
  existing one as a starting point.

## Licensing of your contributions (please read)

GameNest is **open core**:

- The repository is licensed under **AGPL-3.0** (see [`LICENSE`](LICENSE)).
- The `ee/` directory is reserved for future **commercial** features under a
  separate license (see [`ee/LICENSE`](ee/LICENSE)).

To keep this sustainable, we ask every contributor to agree to the following when
they submit a contribution (a lightweight Contributor License Agreement):

> You certify that you wrote the contribution or otherwise have the right to
> submit it (the [Developer Certificate of Origin](https://developercertificate.org/)),
> **and** you license your contribution to The GameNest Authors under **both** the
> AGPL-3.0 **and** the GameNest Commercial License, so that it may be used in the
> open-source core and, where relevant, in commercial features. You retain
> copyright to your contribution.

**How to agree:** sign off your commits with the `-s` flag, which adds a
`Signed-off-by:` line certifying the above:

```bash
git commit -s -m "Fix off-by-one in scheduler tick"
```

> Maintainer note: a [CLA Assistant](https://github.com/cla-assistant/cla-assistant)
> bot should be enabled on the GitHub repo to record this agreement automatically on
> each PR. Until then, the `Signed-off-by` sign-off is required.

## Reporting bugs / asking questions

- **Bugs / features:** open a GitHub issue with steps to reproduce and your OS +
  GameNest version (Settings → About).
- **Security issues:** do **not** open a public issue — see [`SECURITY.md`](SECURITY.md).
- **Help & chat:** the community Discord (linked in the README) is the best place for
  questions. Support there is best-effort; the hosted plan gets priority support.

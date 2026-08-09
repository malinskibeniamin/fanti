# Public-release checklist

This checklist covers publishing Fanti's source. It does not approve a public hosted
service. The deployment warning in [SECURITY.md](../SECURITY.md) remains in force.

## Automated checks

Run from the repository root:

```sh
docker compose config --quiet
```

A clean secret-scanner result lowers risk but cannot prove that a repository contains no
sensitive information.

## Prepare the source

- [ ] Confirm every contributor has the right to license their work under the MIT License.
- [ ] Review images, authored copy, test fixtures, and vendored datasets for redistribution
      rights.
- [ ] Confirm every third-party dataset has a source, pinned version, checksum where
      practical, licence, and attribution in [NOTICES.md](../NOTICES.md).
- [ ] Confirm character frequency ranks are generated only from the attributed Tatoeba
      derivative shipped in `backend/data/downloads/`.
- [ ] Use only synthetic, openly licensed, or public-domain content in examples and test
      artifacts.
- [ ] Run Gitleaks against the final tracked files and the complete history being published.
- [ ] Inspect the final archive manually for environment files, credentials, database dumps,
      uploads, backups, local paths, personal data, and private links.

## Publish a clean repository

Create the public repository from a reviewed `git archive` of the release commit. Initialize
new history in that exported directory. Do not copy `.git`, push with `--mirror`, or change
the visibility of the private development repository.

Choose the final repository owner and name before publishing. If it is not
`malinskibeniamin/fanti`, update the Go module path, imports, source URLs, and documentation
in the exported snapshot. To keep that existing public module path, rename the private
development repository before creating the clean public repository.

Before the first push, confirm the new commit uses an author name and email intended for
public display. Run the automated checks again inside the exported repository.

## Configure GitHub

- [ ] Keep the default workflow token read-only.
- [ ] Enable secret scanning, push protection, Dependabot alerts, and private vulnerability
      reporting.
- [ ] Protect `main` with required CI checks and pull-request review.
- [ ] Review GitHub Pages, webhooks, deploy keys, environments, variables, and Actions
      permissions before enabling them.
- [ ] Do not copy private issues, pull requests, Actions logs, artifacts, or development
      branches into the public repository.

## Final gate

- [ ] Clone the public repository anonymously into an empty directory.
- [ ] Follow only the public README and contribution guide to build and test it.
- [ ] Verify the repository displays the MIT License and the expected security policy.
- [ ] Confirm no live service, database, or admin endpoint became public as a side effect.

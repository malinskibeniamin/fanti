# Security policy

## Supported version

Security fixes target the latest commit on `main`. No released version has a separate
maintenance window yet.

## Report a vulnerability

Use the repository's **Security → Report a vulnerability** flow. Do not open a public issue
for a vulnerability or include secrets, private data, or exploit details in public
discussions.

Include the affected area, reproduction steps, impact, and the smallest safe proof of
concept. Remove personal data and live credentials from the report.

## Deployment warning

Fanti is currently designed for one user on a trusted machine. The API has no
authentication or user isolation. Its library, conversion, and study endpoints can read or
change application data.

The default Docker Compose ports bind to localhost. Do not expose the API, database, or
admin tools to an untrusted network. Authentication, authorization, HTTPS, request limits,
and a reviewed backup and recovery process are release blockers for an internet-facing
deployment.

## Exposed credentials

If a credential reaches Git history or a CI log, revoke or rotate it first. Removing the
visible text or rewriting history does not make the credential safe again.

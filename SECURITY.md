# Security Policy

## Supported versions

We currently support the latest version of Marvin on the `main` branch.

## Reporting a vulnerability

**Please do not open a public GitHub issue for security vulnerabilities.**

To report a security issue, email us at **oss@flashgap.com** with:

- A description of the vulnerability and its potential impact
- Steps to reproduce or a proof of concept
- Any relevant logs or screenshots

We will acknowledge your report within **48 hours** and aim to provide a fix or mitigation within **14 days**, depending on severity. We will keep you informed throughout the process.

Once a fix is available, we will coordinate a disclosure timeline with you. We ask that you do not publicly disclose the issue until we have released a patch.

## Scope

The following are in scope:

- Authentication and authorization bypasses (e.g. webhook signature verification)
- Injection vulnerabilities in GitHub API interactions
- Secrets leakage through logs or API responses

The following are **out of scope**:

- Issues in third-party dependencies (report those upstream)
- Theoretical vulnerabilities without a working proof of concept
- Denial-of-service attacks against a self-hosted instance

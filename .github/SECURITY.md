# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 2.x.x   | :white_check_mark: |
| < 2.0.0 | :x:                |

## Reporting a Vulnerability

We take security seriously. If you discover a security vulnerability, please follow these guidelines:

1. **Do not open a public issue** on GitHub.
2. Report security vulnerabilities by emailing the maintainer at [security@ericslutz.dev](mailto:security@ericslutz.dev).
3. Include as much information as possible:
   - A description of the vulnerability
   - Steps to reproduce the issue
   - Possible impact of the vulnerability
   - Any suggested fixes (if you have them)

We will make every effort to acknowledge your report promptly.

## Security Best Practices

When deploying:

1. **Use secrets management**: Store credentials using Docker secrets or environment variables, never in code
2. **Network isolation**: Run services in a private network
3. **Read-only filesystem**: Mount configuration files as read-only where possible
4. **Non-root user**: The default Docker image runs as a non-root user (UID 1000)
5. **Keep updated**: Regularly update to the latest version to receive security patches

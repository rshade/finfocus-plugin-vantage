# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in the FinFocus Vantage Plugin,
please email security concerns to the maintainers privately. **Do not** open
a public GitHub issue.

### What to Include

- Description of the vulnerability
- Steps to reproduce (if applicable)
- Potential impact
- Suggested fix (if you have one)

## Supported Versions

| Version | Status | End of Life |
|---------|--------|------------|
| 0.1.x | Current | TBD |

## Security Considerations

### Token Management

- API tokens are provided via environment variables
  (`FINFOCUS_VANTAGE_TOKEN`)
- Tokens are never logged or printed
- Authorization headers are redacted in debug output
- Use workspace/cost report tokens with minimal scope

### Data Handling

- Cost data is sensitive business information
- Ensure configs with real tokens are not committed to git
- Use `.env` files locally (in `.gitignore`)
- Sanitize credentials before opening issues

### Dependencies

- Go modules are pinned to specific versions in `go.mod`
- Security updates are prioritized
- Use `go get -u` to update dependencies safely
- Run `govulncheck` before releases

## Best Practices

1. **Never commit credentials** - Use environment variables
2. **Rotate tokens regularly** - Treat like API keys
3. **Use least privilege** - Prefer cost_report tokens over workspace tokens
4. **Enable HTTPS** - When talking to Vantage API
5. **Monitor logs** - Check for unexpected auth failures
6. **Keep Go updated** - Use Go 1.24.7+
7. **Review PRs carefully** - Look for hardcoded secrets

## Security Tools

We use:

- `govulncheck` - Vulnerability scanning
- `golangci-lint` - Static analysis
- Trivy - Container image scanning (on releases)

## Questions

For security questions or concerns not related to a vulnerability, please
open a discussion or contact maintainers privately.

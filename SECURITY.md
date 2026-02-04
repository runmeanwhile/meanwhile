# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 0.1.x   | :white_check_mark: |

## Reporting a Vulnerability

The Meanwhile team takes security issues seriously. We appreciate your efforts to responsibly disclose your findings.

### How to Report

**Please do not report security vulnerabilities through public GitHub issues.**

Instead, please report security vulnerabilities by:

1. **Email**: Send details to darkostanimirovic \[at\] gmail \[dot\] com with the subject line "Meanwhile Security Vulnerability"

2. **Private Security Advisory**: Use GitHub's private security advisory feature at:
   https://github.com/darkostanimirovic/meanwhile/security/advisories/new

### What to Include

Please include as much of the following information as possible:

- Type of vulnerability (e.g., injection, authentication bypass, etc.)
- Full paths of affected source files
- Location of the affected code (tag/branch/commit or direct URL)
- Step-by-step instructions to reproduce the issue
- Proof-of-concept or exploit code (if possible)
- Impact assessment (what an attacker could achieve)
- Any suggested fixes or mitigations

### Response Timeline

- **Initial Response**: Within 72 hours of report submission
- **Status Update**: Within 1 week with our assessment
- **Resolution Timeline**: Varies based on severity
  - Critical: 2-5 days
  - High: 1-2 weeks
  - Medium: 2-6 weeks
  - Low: Best effort

### Security Update Process

1. **Acknowledgment**: We'll confirm receipt and begin investigation
2. **Assessment**: We'll validate the vulnerability and assess severity
3. **Development**: We'll develop a fix (may involve you for verification)
4. **Disclosure**: We'll coordinate disclosure timing with you
5. **Release**: We'll release a patched version
6. **Announcement**: We'll publish a security advisory

### Disclosure Policy

- We ask that you give us reasonable time to fix the issue before public disclosure
- We will acknowledge your responsible disclosure in our security advisory (unless you prefer to remain anonymous)
- We aim for coordinated disclosure within 90 days of report

### Security Best Practices for Users

When using Meanwhile in production:

1. **Keep Dependencies Updated**: Regularly update to the latest version
2. **Secure API Keys**: Never commit LLM provider API keys to version control
3. **Environment Variables**: Store sensitive configuration in environment variables
4. **Input Validation**: Validate and sanitize all user inputs before passing to agents
5. **Rate Limiting**: Implement rate limiting to prevent abuse
6. **Monitoring**: Monitor agent behavior and API usage
7. **Access Control**: Restrict who can configure and run agents
8. **Audit Logging**: Log agent activities for security auditing

### Known Security Considerations

**LLM-Specific Risks:**
- **Prompt Injection**: Validate inputs to prevent malicious prompt manipulation
- **Data Leakage**: Be cautious about what data you expose to LLM providers
- **Cost Control**: Implement budgets/limits to prevent runaway costs
- **Output Validation**: Validate and sanitize LLM outputs before use

**Framework-Level:**
- **Tool Execution**: Tools execute arbitrary code - only use trusted tools
- **Protocol Safety**: Custom protocols can affect agent behavior - review carefully
- **Memory Storage**: Ensure memory stores have appropriate access controls

### Security-Related Configuration

```go
// Example: Secure agent configuration
agent, err := agent.NewBuilder().
    WithID("secure-agent").
    WithProvider(provider). // Use authenticated provider
    WithInstructions("..."). // Avoid exposing sensitive data in instructions
    Build()

// Use environment variables for secrets
apiKey := os.Getenv("OPENAI_API_KEY")
if apiKey == "" {
    log.Fatal("API key not set")
}
```

### Bug Bounty Program

We currently do not offer a paid bug bounty program. However, we deeply appreciate security research and will:
- Acknowledge your contribution
- Credit you in our security advisories (with your permission)
- Provide recognition in our CHANGELOG

### Questions

For questions about this security policy, please open a GitHub issue (for non-sensitive questions) or contact the maintainers directly.

---

**Last Updated**: January 19, 2026

Thank you for helping keep Meanwhile and our users safe!

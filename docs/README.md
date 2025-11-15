# Documentation

This directory contains all project documentation, organized by topic.

## Structure

```
docs/
├── architecture/          # Architecture and design documentation
│   ├── IMPLEMENTATION_PLAN.md     # Complete architecture, design decisions, and migration strategy
│   └── GOLANG_DESIGN.md           # Detailed Golang implementation design
│
├── development/          # Development guides and standards
│   ├── BRANCH_PROTECTION.md       # PR requirements and branch protection setup
│   └── CONCURRENCY_LESSONS.md     # Concurrency patterns and lessons learned
│
├── migration/           # Migration-related documentation
│   └── migration_mapping.md       # State variable mapping from Node-RED to HA
│
├── deployment/          # Deployment and operations guides
│   └── DOCKER.md                  # Docker build, run, and deployment guide
│
└── REVIEW.md           # Code review notes and findings
```

## Quick Links

### Essential Documentation
- **Getting Started**: See [../README.md](../README.md) for project overview
- **Development Guide**: See [../AGENTS.md](../AGENTS.md) for development standards and testing
- **Architecture**: Start with [architecture/IMPLEMENTATION_PLAN.md](./architecture/IMPLEMENTATION_PLAN.md)

### For Developers
- [../AGENTS.md](../AGENTS.md) - Development standards, testing requirements, CI/CD guidelines
- [development/BRANCH_PROTECTION.md](./development/BRANCH_PROTECTION.md) - PR requirements
- [development/CONCURRENCY_LESSONS.md](./development/CONCURRENCY_LESSONS.md) - Concurrency patterns
- [deployment/DOCKER.md](./deployment/DOCKER.md) - Docker deployment

### For Architecture & Design
- [architecture/IMPLEMENTATION_PLAN.md](./architecture/IMPLEMENTATION_PLAN.md) - Complete migration plan
- [architecture/GOLANG_DESIGN.md](./architecture/GOLANG_DESIGN.md) - Detailed design decisions
- [migration/migration_mapping.md](./migration/migration_mapping.md) - State variable mapping

## Root-Level Documentation

Some documentation files remain in the repository root because they are required there:
- **[CLAUDE.md](../CLAUDE.md)** - Claude Code project instructions (must be in root)
- **[AGENTS.md](../AGENTS.md)** - Development guide (must be in root)
- **[README.md](../README.md)** - Project overview (standard location)

## Contributing

When adding new documentation:
1. Place it in the appropriate subdirectory based on its topic
2. Update this README with a link to the new document
3. Update relevant cross-references in other documents
4. Follow markdown best practices (headers, links, code blocks)

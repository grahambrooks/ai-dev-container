# Contributing to devc

Thanks for your interest in contributing!

## Development

### Prerequisites

- Go 1.22+
- Docker
- Make

### Build

```sh
make build
```

### Test

```sh
make test
```

### Lint

```sh
make lint
```

## Submitting changes

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/my-change`)
3. Make your changes and add tests
4. Run `make test && make lint` to verify
5. Commit with a clear message describing what and why
6. Open a pull request

## Reporting issues

Open an issue on GitHub with:

- What you expected to happen
- What actually happened
- Steps to reproduce
- `devc` version and OS

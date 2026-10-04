# incus-apply

> [!NOTE]
> This repository is a fork of [abiosoft/incus-apply](https://github.com/abiosoft/incus-apply).
>
> The fork tracks upstream while adding a narrow embeddable API and native Go integration points needed by downstream consumers such as Aginctus. Changes should stay small, well-tested, and suitable for upstreaming whenever possible. For the canonical upstream project, releases, and documentation, see [abiosoft/incus-apply](https://github.com/abiosoft/incus-apply).

[![Go](https://github.com/abiosoft/incus-apply/actions/workflows/go.yml/badge.svg)](https://github.com/abiosoft/incus-apply/actions/workflows/go.yml)
[![Integration](https://github.com/abiosoft/incus-apply/actions/workflows/integration.yml/badge.svg)](https://github.com/abiosoft/incus-apply/actions/workflows/integration.yml)

Declarative configuration management for [Incus](https://linuxcontainers.org/incus/).

![incus-apply demo](./demo.gif)

## Installation

Install the latest release binary:

```bash
curl -LO https://github.com/abiosoft/incus-apply/releases/latest/download/incus-apply-$(uname)-$(uname -m)
sudo install incus-apply-$(uname)-$(uname -m) /usr/local/bin/incus-apply
```

Or explore other [installation options](https://incus-apply.abiosoft.com/installation).

## Quick Start

### 1. Create a config file `debian.yaml`:

```yaml
kind: instance
name: debian
image: images:debian/12
profiles:
  - default
config:
  limits.cpu: "2"
  limits.memory: 1GiB
```

### 2. Apply it:

```bash
incus-apply debian.yaml
```

## Embeddable Go API

This fork exposes a small library facade at `github.com/abiosoft/incus-apply/apply`.

The module path intentionally remains upstream-compatible. Consumers that want
the fork can pin it with a Go module replacement while the API work is carried
upstream:

```go
require github.com/abiosoft/incus-apply <version>

replace github.com/abiosoft/incus-apply => github.com/blevinn/incus-apply <fork-version>
```

The API accepts configuration through an `io.Reader` and separates planning
from mutation:

```go
client := apply.New(apply.Options{
    Project: "default",
})

preview, err := client.Plan(reader)
if err != nil {
    // handle planning error
}

result, err := client.Execute(reader)
```

`Plan` performs reconciliation discovery and diffing without applying the
changes. `Execute` performs the selected operation non-interactively and
returns the same structured preview alongside the execution result.

The current default backend still preserves upstream's command-backed Incus
behavior. That backend is transitional; callers should depend on the public API,
not command execution details. A native Incus Go backend is planned as the next
fork-specific integration step.

## Documentation

Check the [project website](https://incus-apply.abiosoft.com).

## License

Apache 2.0

## Sponsoring the Project

If you (or your company) are benefiting from the project and would like to support the contributors, kindly sponsor.

- [Github Sponsors](https://github.com/sponsors/abiosoft)
- [Buy me a coffee](https://www.buymeacoffee.com/abiosoft)


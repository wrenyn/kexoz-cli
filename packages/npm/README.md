# Kexoz CLI

Share local web apps and TCP/UDP services from your terminal. This package installs
the native CLI from the matching [GitHub release](https://github.com/wrenyn/kexoz-cli/releases)
and checks its SHA-256 against that release's `SHA256SUMS` before installing it.

```sh
npm install -g @kexoz/cli
kexoz --version
kexoz login
kexoz http 3000
```

Requires Node.js 20 or newer, access to GitHub Releases, and npm install scripts
enabled. Supports Windows, macOS and Linux on x64 and ARM64. Keep your local app
and Kexoz running while sharing. Press Ctrl+C to stop.

If your npm configuration disables install scripts, reinstall with
`npm install -g @kexoz/cli --ignore-scripts=false` after allowing the package's
install script in your organization's policy.

See the [CLI documentation](https://github.com/wrenyn/kexoz-cli#readme) for commands,
API token authentication, projects, path routing, and self-hosted gateways.

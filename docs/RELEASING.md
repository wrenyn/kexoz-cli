# CLI publishing setup

The source is prepared for npm, Homebrew, Chocolatey and direct installers.
Package names in the website are not proof that the corresponding registry has
been configured. Verify each public installation before advertising it.

## 1. Publish the native release first

Commit and push the CLI changes, choose an **unused** stable `vX.Y.Z` tag, and
push the tag. `v1.0.7` is the example used here; check existing releases first.
The release workflow tests Go and the npm wrapper on Windows, Linux and macOS,
builds x64 and ARM64 for all three, and uploads all six binaries plus
`SHA256SUMS`. Optional registry jobs run only after that release succeeds.

Check the release is public at https://github.com/wrenyn/kexoz-cli/releases and
that all seven assets are present. The new website installers require
`SHA256SUMS`; **publish this release before deploying the website changes**.
Do not reuse or move an existing tag. The manual Release CLI action accepts an
existing stable tag for retrying a failed run.

## 2. npm: reserve the scope and publish the first package

1. Create/sign into an account at https://www.npmjs.com/ and enable 2FA.
2. Own the `kexoz` npm username or create the `kexoz` organization, and give your
   account permission to publish under `@kexoz`. GitHub's `wrenyn` organization
   does not automatically grant ownership of the npm scope.
3. After the native release above exists, run from `cli/packages/npm`:

   ```powershell
   npm login
   npm version 1.0.7 --no-git-tag-version --allow-same-version --ignore-scripts
   npm ci --ignore-scripts
   npm test
   node install.js
   node bin/kexoz.js --version
   npm pack --dry-run
   npm publish --access public
   ```

   Replace `1.0.7` with the exact version of your published GitHub release.
   **`npm pack` only makes a tarball; `npm publish` uploads it to the registry.**
   The wrapper downloads the binary for its own package version, never latest.
   It requires Node.js 20+, install scripts enabled, and access to GitHub.

4. In the package's npm settings, add a GitHub Actions trusted publisher:

   | Field | Value |
   | --- | --- |
   | Organization/user | `wrenyn` |
   | Repository | `kexoz-cli` |
   | Workflow filename | `release.yaml` |
   | Environment | Leave empty |
   | Allowed actions | Enable direct `npm publish` |

5. In `wrenyn/kexoz-cli` GitHub Settings > Secrets and variables > Actions,
   add the **repository variable** `NPM_PUBLISH_ENABLED=true`.
   Future tags publish through OIDC with Node.js 24; no npm token is required.
   Complete the first trusted publish promptly; npm currently expires a newly
   configured publisher if it has not published within two days.
   If using token authentication instead, the workflow also accepts the
   `NPM_TOKEN` secret with appropriate package write permissions.
6. On a separate clean machine, verify:

   ```sh
   npm install -g @kexoz/cli
   kexoz --version
   kexoz login
   kexoz http 3000
   ```

Reference: https://docs.npmjs.com/trusted-publishers/ and
https://docs.npmjs.com/creating-and-publishing-scoped-public-packages/.

## 3. Homebrew: create the tap

1. Create a **public**, initialized `wrenyn/homebrew-tap` GitHub repository.
2. Add a fine-grained GitHub token as `HOMEBREW_TAP_TOKEN` in `kexoz-cli`.
   It needs Contents and Pull requests read/write on **homebrew-tap**.
3. Set the `HOMEBREW_PUBLISH_ENABLED=true` repository variable in `kexoz-cli`.
4. Run Update Homebrew Tap manually for the published tag, or push the next
   version tag. It verifies all six release assets before generating the four
   macOS/Linux URL and SHA-256 pairs. It opens a PR in the tap.
5. **Merge that PR.** The command is available only after the formula is merged.
6. Verify on supported macOS and Linux hosts:

   ```sh
   brew update
   brew install wrenyn/tap/kexoz
   brew test kexoz
   kexoz --version
   ```

The source no longer contains a formula with fake checksums. Generate locally
with `node scripts/prepare-packages.mjs v1.0.7 downloaded-assets .release-packages`.
Generated packages go into `.release-packages`; do not publish the templates.

## 4. Chocolatey: submit and wait for public approval

1. Create an account at https://community.chocolatey.org/ and confirm you can
   own/publish the `kexoz` package ID.
2. Store its API key as `CHOCOLATEY_API_KEY` in the CLI repository's Actions secrets.
3. Set the `CHOCOLATEY_PUBLISH_ENABLED=true` repository variable.
4. Run Publish Chocolatey Package manually with the published CLI tag, or push
   the next version tag. It generates checksums, packs the portable executable
   package, smoke-tests install/version/uninstall, then submits it.
5. Complete Chocolatey's moderation process. A successful submission does not
   mean `choco install kexoz` is publicly available yet.
6. From an administrator PowerShell on a clean Windows machine:

   ```powershell
   choco install kexoz -y
   kexoz --version
   choco uninstall kexoz -y
   ```

Reference: https://docs.chocolatey.org/en-us/create/functions/get-chocolateywebfile/.

## 5. Direct installers, Docker, and website rollout

After publishing the native release, deploy the website and verify these URLs
return script content, not an HTML page or redirect to a sign-in page:
https://kexoz.dev/install.sh and https://kexoz.dev/install.ps1.
Both installers pin a single release, verify SHA-256, and support x64/ARM64.
PowerShell updates the current shell and user PATH. Unix installs to
`~/.local/bin`; users may need to add that directory to their shell's PATH.
Optional `KEXOZ_VERSION=vX.Y.Z` selects a release;
`KEXOZ_CLI_REPO` and `KEXOZ_INSTALL_DIR` override source and destination.
`KEXOZ_NO_MODIFY_PATH=1` suppresses PowerShell PATH changes for managed installs.

GHCR uses `GITHUB_TOKEN`; set the `kexoz` package visibility to **Public** once
in https://github.com/orgs/wrenyn/packages/container/kexoz/settings. Check
`docker pull ghcr.io/wrenyn/kexoz:latest` while logged out. Native Linux supports
`--network host`; Docker Desktop requires its host-networking feature enabled.
Persist `/root/.config/kexoz` in a volume if the container should retain login.

Only after each package passes a public clean install, set its website build
variable and redeploy:

| Website environment variable | Enable after |
| --- | --- |
| `NEXT_PUBLIC_CLI_NPM_AVAILABLE=true` | npm install/version/login/share passes |
| `NEXT_PUBLIC_CLI_HOMEBREW_AVAILABLE=true` | Tap PR merged and brew install/test passes |
| `NEXT_PUBLIC_CLI_CHOCOLATEY_AVAILABLE=true` | Community approval and choco install passes |

These flags default to false. They control advertising, not publishing.
The CLI repository's `*_PUBLISH_ENABLED` variables independently control jobs.

For each supported OS, start a local server on port 3000, run `kexoz login`,
then `kexoz http 3000`. Open the printed HTTPS URL from another device and
check the response. Stop with Ctrl+C. Also exercise TCP/UDP and project commands
you advertise before treating those capabilities as production-verified.

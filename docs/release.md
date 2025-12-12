# Release

## GitHub Release Artifacts

`wacli` uses GoReleaser (`.goreleaser.yaml`) and the GitHub Actions workflow `.github/workflows/release.yml`.

To cut a release:

1. Tag and push:
   - `git tag vX.Y.Z`
   - `git push origin vX.Y.Z`
2. Wait for the GitHub Actions “Release” workflow to publish the release artifacts.

Expected macOS artifact name (used by the tap updater):

- `wacli-macos-universal.tar.gz`

## Homebrew Tap

The tap formula lives in `../homebrew-tap/Formula/wacli.rb`.

Once a release exists, update the tap formula by running the `Update Formula` workflow in the tap repo with:

- `formula`: `wacli`
- `tag`: `vX.Y.Z`
- `repository`: `steipete/wacli`


# Release exmd

Run the commands from the repository root in the same Bash session on macOS
or Linux. Use the Go version in [go.mod](../go.mod), Git, `tar`, `shasum`,
and GitHub CLI authenticated with write access to `michaelishri/exmd`.

## Bump the version

Choose the next application version. For example, for version `0.1.1`,
set `const version = "0.1.1"` in [cmd/exmd/main.go](../cmd/exmd/main.go).
The binary reports `0.1.1` and the Git tag is `v0.1.1`.

Commit the bump and land it on `main` with the changes you want to release.
Start from a clean checkout of that commit. Replace the example version
below with the one you chose.

```bash
set -euo pipefail

repo=michaelishri/exmd
version=0.1.1
tag="v$version"

git fetch origin main
test -z "$(git status --porcelain)"
commit="$(git rev-parse HEAD)"
git merge-base --is-ancestor "$commit" origin/main
remote_tag="$(git ls-remote --tags origin "refs/tags/$tag")"
test -z "$remote_tag"
```

## Test and package the binaries

Run the test suite and build one archive per platform. Each archive has
an executable named `exmd` at its root. Keep the archive names below so
the Homebrew formula can use the same names when it moves to the new version.

```bash
go test ./...

release_dir="$(mktemp -d "${TMPDIR:-/tmp}/exmd-$tag.XXXXXX")"
for os in darwin linux; do
  for arch in amd64 arm64; do
    asset="exmd-$os-$arch"
    mkdir "$release_dir/$asset"
    CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
      go build -trimpath -o "$release_dir/$asset/exmd" ./cmd/exmd
    tar -czf "$release_dir/$asset.tar.gz" -C "$release_dir/$asset" exmd
  done
done

(
  cd "$release_dir"
  shasum -a 256 exmd-*.tar.gz > SHA256SUMS
  shasum -a 256 -c SHA256SUMS
)

host="exmd-$(go env GOHOSTOS)-$(go env GOHOSTARCH)"
smoke_dir="$(mktemp -d "${TMPDIR:-/tmp}/exmd-smoke.XXXXXX")"
tar -xzf "$release_dir/$host.tar.gz" -C "$smoke_dir"
test "$("$smoke_dir/exmd" --version)" = "exmd version $version (spec v1)"
"$smoke_dir/exmd" check internal/doc/testdata/valid-flow.md
```

## Publish and verify the GitHub release

Write release notes to `$release_dir/release-notes.md`. Summarize what
changed for users and call out any breaking changes. GitHub creates the
tag at the commit you built and attaches the four archives and their
SHA-256 checksums.

```bash
gh release create "$tag" \
  "$release_dir"/exmd-*.tar.gz "$release_dir/SHA256SUMS" \
  --repo "$repo" --target "$commit" --title "exmd $tag" \
  --notes-file "$release_dir/release-notes.md"
```

For a prerelease, include the suffix in the application version and tag
and add `--prerelease` to the publish command.

Fetch the published tag and verify the assets downloaded from GitHub.

```bash
git fetch origin tag "$tag"
test "$(git rev-parse "$tag^{commit}")" = "$commit"

verify_dir="$(mktemp -d "${TMPDIR:-/tmp}/exmd-download.XXXXXX")"
gh release download "$tag" --repo "$repo" --dir "$verify_dir" \
  --pattern 'exmd-*.tar.gz' --pattern SHA256SUMS
cmp "$release_dir/SHA256SUMS" "$verify_dir/SHA256SUMS"
(
  cd "$verify_dir"
  shasum -a 256 -c SHA256SUMS
)
gh release view "$tag" --repo "$repo" --json url,isDraft,isPrerelease
```

Use the verified release URLs and checksums when updating the
[Homebrew tap](https://github.com/michaelishri/homebrew-tap).

## What changed and why

<!-- One purpose per pull request. Link the issue; a big change should have
been discussed in one first (see CONTRIBUTING.md). -->

## How it was tested

<!-- The commands you ran and their result, plus anything you tried by hand
(a TV, a browser display). -->

## What could break

<!-- Behaviour, data, the API or the database that this touches. "Nothing" is
a fine answer if it is true. -->

## Checklist

- [ ] `gofmt -l .`, `go vet ./...` and `go test ./...` pass
- [ ] `npm run check` and `npm test` in `web/` pass, and `npm run e2e` if the web interface changed
- [ ] New behaviour has tests, and a bug fix has a test that fails without it
- [ ] Documentation and `CHANGELOG.md` (Unreleased) are updated where people will notice the change
- [ ] No secrets, tokens or personal data (home IP addresses, device names) in the diff
- [ ] `THIRD_PARTY_LICENSES` is updated if a dependency was added or changed
- [ ] The change follows [CODE_GUIDELINES.md](../CODE_GUIDELINES.md) and contains nothing unrelated to this purpose

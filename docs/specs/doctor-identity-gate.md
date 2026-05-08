<!-- runx-public-repo-gate: allow-file personal_path_id — spec describes the identity gate using the literal personal-stack identifiers it detects -->

# Spec: `cursor-tools doctor identity` — GitHub Identity Gate

> **Status**: Draft (v256 W1 P0 #8)
> **Owner**: nfsarch33
> **Sprint**: v256 (slated for W1 implementation)
> **Closes**: PM-12 finding (initial), PM-16 escalation (4-token confirmation), PM-17 / PM-18 reaffirmation (workaround `/tmp/p` still required at PM-18 for every personal-repo `gh`/`git push` invocation)

## Problem statement

The macbook shell session re-injects four GitHub tokens at every new terminal startup:

- `GITHUB_TOKEN`
- `GITHUB_API_TOKEN`
- `HOMEBREW_GITHUB_API_TOKEN`
- `VENDIR_GITHUB_API_TOKEN`

These come from a Zendesk-managed source (likely `~/.zshrc`, a 1Password CLI integration, or a corp keychain script). When any are present, `gh` and `git push` use the Zendesk identity (`jlianzendesk`) instead of the personal one (`nfsarch33`), even after `gh auth switch -h github.com -u nfsarch33`. This silently violates the user's hard rule:

> **No Zendesk GitHub id is allowed for personal/private projects. `nfsarch33` is the only GitHub id that we should be using for all personal/private repos.**

Workaround in active use since PM-12: a wrapper script at `/tmp/p` runs `env -u GITHUB_TOKEN -u GITHUB_API_TOKEN -u HOMEBREW_GITHUB_API_TOKEN -u VENDIR_GITHUB_API_TOKEN "$@"` and is prefixed before every `gh`/`git push` against personal repos. This is fragile: easy to forget, not portable, and a fresh shell loses it.

The CI workflow already enforces "nfsarch33 author + zero zendesk leaks" *post-commit*. We need the equivalent gate **pre-action** in the local toolchain so an agent or human cannot mint a Zendesk-tainted commit in the first place.

## Goals

1. **Pre-action, fail-closed gate** — `cursor-tools doctor identity` must return non-zero before any `gh`/`git push` against a personal-repo remote when identity is unsafe.
2. **Auto-scrub** — the cursor-tools wrapper that subprocesses `gh`/`git` against `git@github-agtc:nfsarch33/*` remotes must `env -u` the four tokens automatically, so the user / agent doesn't need `/tmp/p`.
3. **Daily routine integration** — `daily-startup-prompt.md` Phase 0 already runs `~/bin/cursor-tools doctor mcp` and `doctor resume`; add `doctor identity` to the routine so identity drift is caught at session start.
4. **Token-leak reporting** — when the gate fails, the output must list exactly which token is leaking from where (env-var name + best-effort source: `~/.zshrc`, `~/.bashrc`, `~/.config/op/`, or unknown), so the user can fix it permanently.

## Non-goals

- Block Zendesk identity globally — `jlianzendesk` is still legitimate inside Zendesk repos; the gate scopes per-remote or per-CWD.
- Manage token rotation or vault sync — that lives in `op-1Password-fleet.md`.
- Replace the CI gate — the CI gate is the durable guarantee; this gate is a fast feedback loop.

## Design

### CLI surface

```text
$ cursor-tools doctor identity [--json] [--strict]
```

- New profile `"identity"` joins `install`, `mcp`, `platform`, `deps`, `resume`, `stack`, `drl` in `internal/cli/doctor.go`.
- `doctor` (no profile) — runs `BuildAllSuites` which already includes the identity suite when present in the catalog.

### Assertion list

A new suite `"GitHub Identity"` is added to `internal/health/suites.go` (insert next to `suiteGitSync` so the existing email check is reused without duplication).

| # | Assertion | Method | Severity |
| :-: | :--- | :--- | :---: |
| 1 | `GITHUB_TOKEN` env var unset | `os.LookupEnv` | hard |
| 2 | `GITHUB_API_TOKEN` env var unset | `os.LookupEnv` | hard |
| 3 | `HOMEBREW_GITHUB_API_TOKEN` env var unset | `os.LookupEnv` | hard |
| 4 | `VENDIR_GITHUB_API_TOKEN` env var unset | `os.LookupEnv` | hard |
| 5 | `gh auth status` shows active account = `nfsarch33` | shell `gh auth status` parse | hard |
| 6 | Active host token in `~/.config/gh/hosts.yml` is for `nfsarch33` | YAML read | hard |
| 7 | `git config --global user.email` ∈ `{jaslian@gmail.com, nfsarch33@users.noreply.github.com}` | `git config` | hard |
| 8 | Per-repo `git config user.email` (when run inside a repo) ∈ same allow-list | `git -C <cwd> config` | soft (warn-only when CWD is not a personal repo) |
| 9 | Current repo's `origin` URL — when matches `git@github-agtc:nfsarch33/*` or `https://github.com/nfsarch33/*` — is annotated as personal | `git remote get-url` | soft |
| 10 | When CWD is a personal repo (per #9) AND any of #1–4 is set: emit `LEAKED token=<name> likely_source=<file>` line | scan `~/.zshrc`, `~/.bashrc`, `~/.config/op/`, `~/.profile`, `/etc/profile.d/*` for the token name | hard (fails if any) |
| 11 | `~/bin/cursor-tools` symlink resolves to a personal-built binary, not a Zendesk-tainted one | `readlink -f` + sha256 of binary vs known-good registry | soft |

### Plug-in point

```go
// internal/health/suites.go
func BuildDoctorSuites(p config.Paths, profile string) []*Suite {
    var names []string
    switch profile {
    // ... existing profiles ...
    case "identity":
        names = []string{
            "GitHub Identity",
            "Git Sync",                     // reuses existing email check
            "Pre-Push Readiness",           // already runs pre-push hook
        }
    // ...
    }
    return buildSuiteList(p, names)
}

// And add the suite spec to suiteCatalog:
var suiteCatalog = []suiteSpec{
    // ...
    {name: "GitHub Identity", build: suiteGitHubIdentity},
    // ...
}

func suiteGitHubIdentity(p config.Paths) *Suite {
    s := &Suite{Name: "GitHub Identity"}
    leakedTokens := []string{}
    for _, name := range []string{"GITHUB_TOKEN", "GITHUB_API_TOKEN", "HOMEBREW_GITHUB_API_TOKEN", "VENDIR_GITHUB_API_TOKEN"} {
        if _, set := os.LookupEnv(name); set {
            leakedTokens = append(leakedTokens, name)
        }
        s.Assert(name+" env var unset", !set, name+" must not be set in personal-repo shells")
    }

    // gh auth status active user
    out, _ := exec.Command("gh", "auth", "status").CombinedOutput()
    isPersonalActive := strings.Contains(string(out), "Active account: true") && strings.Contains(string(out), "nfsarch33")
    s.Assert("gh auth active = nfsarch33", isPersonalActive, "see `gh auth status` and `gh auth switch -h github.com -u nfsarch33`")

    // hosts.yml scan
    hostsYAML, _ := os.ReadFile(filepath.Join(os.Getenv("HOME"), ".config/gh/hosts.yml"))
    s.Assert("gh hosts.yml has nfsarch33 token", strings.Contains(string(hostsYAML), "nfsarch33"), "no nfsarch33 token entry")

    // git global email
    email, _ := gitOutput("", "config", "--global", "user.email")
    allowed := strings.Contains(email, "jaslian@gmail.com") || strings.Contains(email, "nfsarch33@")
    s.Assert("git global user.email is personal", allowed, "expected jaslian@gmail.com or nfsarch33 noreply, got "+email)

    // Token-source scan (best effort) only when leaks exist
    if len(leakedTokens) > 0 {
        for _, token := range leakedTokens {
            src := scanShellRcFiles(token)
            s.Fail("LEAKED "+token, "likely source: "+src+" — see docs/specs/doctor-identity-gate.md § Remediation")
        }
    }
    return s
}
```

### Auto-scrub wrapper

The `cursor-tools` binary already has a `subprocess` helper for invoking `git`/`gh`. Wrap that helper to:

1. When the target remote URL or the resolved repo path matches `nfsarch33/*` → run via `env -u GITHUB_TOKEN -u GITHUB_API_TOKEN -u HOMEBREW_GITHUB_API_TOKEN -u VENDIR_GITHUB_API_TOKEN`.
2. When the remote is a Zendesk repo → leave env unchanged.
3. When ambiguous (no remote yet, e.g. `git init`) → fail closed with a clear message asking the user to declare scope.

Concretely a new helper in `internal/cli/exec.go`:

```go
// runGitHubAware runs argv[0] with auto-scrubbed env when the target appears to be
// a personal-repo operation. Returns ErrAmbiguousScope if scope cannot be inferred.
func runGitHubAware(cwd string, argv []string) (*exec.Cmd, error) { ... }
```

All `gh`/`git push` callers in `cursor-tools` route through this helper.

### Pre-push hook integration

The existing pre-push hook (already in `internal/health/suites.go` as "Pre-Push Readiness") should additionally invoke `doctor identity` and refuse to push if any hard assertion fails. This blocks the human path; the cursor-tools wrapper blocks the agent path.

## Test plan

### Unit tests (`suites_identity_test.go`)

| # | Case | Setup | Expected |
| :-: | :--- | :--- | :--- |
| 1 | All four tokens unset, gh auth = nfsarch33, git email personal | clean env | suite passes |
| 2 | `GITHUB_TOKEN` set | `t.Setenv("GITHUB_TOKEN", "x")` | fail on assertion #1 |
| 3 | All four tokens set | setenv all four | 4 hard fails + 4 LEAKED lines |
| 4 | gh active account = jlianzendesk | mock `gh auth status` output | fail on assertion #5 |
| 5 | Per-repo email is jlianzendesk@zendesk.com inside personal repo | tempdir + `git init` + `git config user.email` | fail on assertion #8 |
| 6 | Token-source scan finds GITHUB_TOKEN line in mock `~/.zshrc` | tempdir HOME + write fake .zshrc | LEAKED line includes `~/.zshrc` |
| 7 | JSON output mode | `--json` flag | structured output with `assertions[]` array |

### Integration test (`scripts/test-doctor-identity-acceptance.bats`)

```bash
@test "doctor identity is green in a clean shell" {
  env -i HOME="$HOME" PATH="$PATH" cursor-tools doctor identity
}

@test "doctor identity fails when GITHUB_TOKEN is set" {
  GITHUB_TOKEN=fake_token run cursor-tools doctor identity
  [[ "$status" -ne 0 ]]
  [[ "$output" == *"GITHUB_TOKEN env var unset"* ]]
  [[ "$output" == *"FAIL"* ]]
}

@test "doctor identity scrubs tokens before invoking gh" {
  GITHUB_TOKEN=fake cursor-tools doctor identity --scrub-and-run
  # When --scrub-and-run is set, the gate should self-heal by env -u and re-run gh
}
```

### Coverage gate

- `suites_identity_test.go` ≥ 0.85 statement coverage (no skip-for-platform branches; the assertions are all OS-agnostic).
- `internal/cli/exec.go` (`runGitHubAware`) ≥ 0.80 with a mock `git remote get-url` + table-driven cases.

### CI gate

- `cursor-tools doctor identity` is run inside the `nfsarch33 author + zero zendesk leaks` job in `.github/workflows/ci.yml`. (Already runs `gitleaks`; this adds a complementary local-policy gate.)

## Remediation playbook

When the gate fails with `LEAKED GITHUB_TOKEN likely_source=~/.zshrc`, the user runs:

```bash
# 1. Inspect the offending line.
rg -n GITHUB_TOKEN ~/.zshrc ~/.bashrc ~/.config/op/ ~/.profile

# 2. Move the line into a Zendesk-only profile, e.g. ~/.zshrc.d/zendesk.zsh,
#    then source that file ONLY when entering a Zendesk repo (use `direnv` or
#    a CWD-conditional block).

# 3. Restart the terminal and re-run the gate.
cursor-tools doctor identity
```

Long-term: migrate Zendesk credentials behind `direnv` `.envrc` files scoped to Zendesk repo trees, so `cd ~/zendesk-repo` triggers the load and `cd ~/Code/global-kb` automatically scrubs.

## Acceptance criteria

- [ ] `cursor-tools doctor identity` exits 0 on a clean shell.
- [ ] `cursor-tools doctor identity` exits non-zero when any of the 4 tokens is set.
- [ ] Output JSON-mode emits `{ "suite": "GitHub Identity", "assertions": [{ "name": ..., "passed": ..., "detail": ... }] }`.
- [ ] `runGitHubAware` auto-scrubs env when target is `nfsarch33/*`.
- [ ] Coverage gates (suite + exec) ≥ thresholds above.
- [ ] `daily-startup-prompt.md` Phase 0 routine includes `cursor-tools doctor identity` step.
- [ ] Pre-push hook calls the gate and refuses to push on hard fail.
- [ ] At least one bats acceptance test runs the full sad-path (token leaked → gate fails) end-to-end in CI.

## Risks / open questions

1. **`gh auth status` parsing fragility** — gh changes its output format occasionally. Mitigation: prefer `gh api user --jq .login` for the active-user identity check; fall back to text parsing only if that fails.
2. **`hosts.yml` schema** — the YAML structure is gh-CLI internal. Use `gh auth token --hostname github.com` instead, then `gh api user --header "Authorization: token <...>"` to verify ownership.
3. **Source-scan false positives** — the LEAKED-source heuristic just greps shell rc files for the token name. May surface stale references. Mitigation: present as `likely source` (already in spec), not authoritative.
4. **Cross-platform behaviour** — Windows native shells (PowerShell + Cursor terminal profile) have different env-var semantics. The check itself works (Go `os.LookupEnv` is OS-portable), but the **fix** path differs (env vars come from `$PROFILE` / Group Policy). Need a Windows-specific remediation guide in the doc when the gate trips on Windows native.

## Implementation timeline (v256)

- **W1**: this spec lands as a doc PR. (Unit test scaffolding + suite skeleton can land alongside if budget allows.)
- **W2**: full implementation + bats acceptance + CI integration.
- **W2 close**: gate enforced fleet-wide; `/tmp/p` workaround retired and tracked in PM-19 retro.

## Related work

- ADR-0004 (`docs/adr/adr-0004-fleet-onboarding-canonical-paths.md`) — sister-doc that formalises the fleet onboarding path; identity gate guards both Path A (USB) and Path B (Tailscale) at the controller side.
- `sop/secrets-1password-fleet.md` — long-term home for the direnv-scoped Zendesk credentials remediation.
- v256 W1 P0 #8 backlog item — this spec satisfies the spec deliverable; implementation closes the backlog.

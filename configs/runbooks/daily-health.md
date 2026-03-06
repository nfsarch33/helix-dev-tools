# Daily Health Check

Run this at the start of each day or session to verify system integrity.

## Steps

- command: cursor-tools health-check
  expect_exit: 0
  description: Run 19-suite integration health check

- command: cursor-tools selftest
  expect_exit: 0
  description: Run 94 hook unit test assertions

- command: cursor-tools sync-counts --apply
  expect_exit: 0
  description: Verify and fix skill/hook counts in index files

- command: cursor-tools promote --dry-run
  expect_exit: 0
  description: Check for pending learning promotions

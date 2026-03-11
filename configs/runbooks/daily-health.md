# Daily Health Check

Run this at the start of each day or session to verify system integrity.

## Steps

- command: cursor-tools memory-routine
  expect_exit: 0
  description: Export parity and memory KPI evidence, then sync durable docs

- command: cursor-tools health-check
  expect_exit: 0
  description: Run the full integration health check suite after evidence refresh

- command: cursor-tools sync-counts --apply
  expect_exit: 0
  description: Verify and fix skill/hook counts in index files

- command: cursor-tools promote --dry-run
  expect_exit: 0
  description: Check for pending learning promotions

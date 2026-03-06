# Session Start

Run at the beginning of each Cursor session.

## Steps

- command: cursor-tools sync-counts --apply
  expect_exit: 0
  description: Ensure all index counts match disk

- command: cursor-tools promote --dry-run
  expect_exit: 0
  description: Check for pending promotions from last session

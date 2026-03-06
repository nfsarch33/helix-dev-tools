# Session End

Triggered automatically by the housekeeping stop hook.

## Steps

- command: cursor-tools sync-counts --apply
  expect_exit: 0
  description: Final count sync

- command: cursor-tools promote
  expect_exit: 0
  description: Promote any pending learnings

- command: git -C ~/Code/global-kb add -A && git -C ~/Code/global-kb commit -m "auto: session sync" && git -C ~/Code/global-kb push
  expect_exit: 0
  description: Commit and push unified-memory repo

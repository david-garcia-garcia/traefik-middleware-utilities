# Standards

1. [hard] Leave a trail — `openspec/specs/std_go_reclaim_value-lifecycle/spec.md:3` — Purpose now says sleep/wake/close are optional Hooks funcs, but the live requirement at `:80` still requires optional interfaces and type-switch lookups on the stored value
   → Revert the live Purpose edit until archive folds the delta, or fold the live requirements so they match Hooks
   Status: done
   Argument: Reverted live Purpose to master's "optional interfaces" wording so it matches the still-unarchived requirement at :80.

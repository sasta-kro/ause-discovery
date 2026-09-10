# History rewrite map, 2026-09-05

The repository owner rewrote Git history on 2026-09-05 with `git filter-repo` to remove `Co-Authored-By: Claude Code` message trailers from commits produced by the implementation session. Authors, dates, file trees, and all commits before `e4e1f00` are unchanged; every commit from `e4e1f00` onward received a new hash. Use this map to correlate older reports, reviews, and conversations that cite the old hashes.

| Old | New | Subject |
| --- | --- | --- |
| `e4e1f00` | `c60b8b4` | Implemented administrator audit visibility |
| `2fa316b` | `69b5de7` | Corrected audit actor normalization, retry, and memory routing |
| `07c4de1` | `a028e82` | Documented the CI and operator readiness increment |
| `e507e02` | `e104774` | Added CI and operator readiness |
| `45154a1` | `27ecdd0` | Corrected CI branch and image metadata contracts |
| `1eb1b9e` | `dcc1867` | Advanced the accepted implementation checkpoint |
| `4fe24f6` | `bd9be46` | Documented the product coherence increment |
| `5876bab` | `97843d7` | Completed product coherence and web hardening |
| `5f766f5` | `be5081d` | Corrected list paging and form recovery contracts |
| `752a556` | `6136c9b` | Corrected reload, dialog recovery, focus, and cursor bounds |
| `526caac` | `f43a9b2` | Corrected record navigation, cursor serialization bound, and stale recovery state |
| `f1548ea` | `dd8df05` | Corrected reload ownership and lifecycle reload recovery |
| `be7e0c1` | `1cc5dc6` | Completed technical MVP verification |
| `9c97ba9` | `6dfab2a` | Recorded technical MVP completion |

All commits before `e4e1f00`, including `722a6ff`, `2460a7f`, `b9a8a9a`, and the foundation history, keep their original hashes.


## Second rewrite, 2026-09-09: deployment-log purge

The VM deployment log was removed from all Git history before the first push beyond `513af4f`, so its contents never reached any remote. Paths removed: `docs/dev-notes/vm-deployment-log.md` and `.memory/sasta-dev-notes/vm-deployment-log.md`. Commits `1bcb830` and `5938c15` touched only that file and were pruned entirely. Mapping for the remaining affected commits:

| Old | New | Subject |
| --- | --- | --- |
| `49e0700` | `9b74a14` | recorded full corpus extraction status in memory |
| `3391597` | `086e5c7` | Wrote professional README with technology badges |
| `8077082` | `61c4b61` | updated readme to this point |
| `b282412` | `7808b24` | Documented internal and public documentation boundaries |
| `b66cfad` | `bf3f338` | restructere memory |

All commits before `49e0700` (including `513af4f` on the remote) keep their hashes, so pushing remains a fast-forward. Backup bundle: `ause-discover-pre-leak-fix.bundle`.

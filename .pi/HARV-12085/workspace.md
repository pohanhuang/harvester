# Plan

Address Vicente-Cheng's review comments on PR #12122.

# Todo

- [x] #1 `roleSetup` empty role → default to `RoleDefault` (`util.go:559`)
- [x] #2 `configureInstalledNode` missing `applyInstallProfile` → added before `roleSetup`
- [x] #3 Retry too short → `retryWithBackoff` with exponential backoff+jitter; 3×2s TUI, 30×5s automation, 60s cap
- [x] #4 mid-body decode not retried → `permanentError` type; body read errors retry, JSON decode errors don't
- [ ] #5 `getInstallProfile` 404 ambiguity → retry on 404 short limit before treating as absent
- [ ] #6 `diskCount > 1` wrong condition → show skip panel for single-disk too
- [ ] #7 Setting validator missing → add `strconv.ParseBool` validator for `skip-longhorn-default-disk`
- [ ] #8 `InsecureSkipVerify` exposes join token → heavy lift, defer / document risk
- [x] #9 Unused `storageClasses`/`storageClassCache` in `handler.go` → deleted

# Next

Fix #6 (`diskCount > 1`), then #5 (404), then #7 (validator).

# PR #12122 — Issues to Address

---

## 1. `roleSetup` returns early when `c.Role == ""`, never sets `create-default-disk` label

**File:** `pkg/installer/console/util.go:558`  
**Problem:** PXE / config-url installs often omit `role`. Early return skips the `node.longhorn.io/create-default-disk` label. With `createDefaultDiskLabeledNodes: true` in the chart, Longhorn creates no default disk on that node even when `SkipLonghornDefaultDisk` is false.  
**Fix:** Treat empty `role` as `RoleDefault` (set the label), then continue; or at minimum set the label before returning.

---

## 2. `configureInstalledNode` never fetches the install profile before `roleSetup`

**File:** `pkg/installer/console/util.go:902`  
**Problem:** Already-installed nodes that call `configureInstalledNode` never call `applyInstallProfile`. Local `SkipLonghornDefaultDisk` stays `false`, so `roleSetup` sets `create-default-disk = true` even on clusters that skipped the default disk.  
**Fix:** Call `applyInstallProfile(hvstConfig)` before `roleSetup` in `configureInstalledNode`, mirroring what `doInstall` does at line 589.

---

## 3. Retry window for `applyInstallProfile` is too short for automation

**File:** `pkg/installer/console/util.go:119`  
**Problem:** Max 3 attempts × 2 s = 6 s total. Join nodes can start in parallel with the management node; cluster readiness may take several minutes. Automation stalls out and leaves the node without the correct disk label.  
**Fix:** Keep the short retry (3 × 2 s) for interactive TUI mode; use a much longer window (e.g. 30 retries × 10 s = 5 min) when `hvstConfig.Install.Automatic` is `true`.

---

## 4. `getInstallProfile` never retries on mid-body decode failure

**File:** `pkg/installer/console/util.go:147`  
**Problem:** `json.Decode` errors (including a connection drop after HTTP 200) return `retry=false`. The request-side transport error retries, so a mid-body drop stops the join while a request-side drop does not. This asymmetry leaves the node unconfigured.  
**Fix:** Read `resp.Body` into a `[]byte` first. Return body-read errors with `retry=true`; return JSON-decode errors with `retry=false`.

---

## 5. `404` status ambiguity — missing endpoint vs. cluster not ready

**File:** `pkg/installer/console/util.go:139`  
**Problem:** `http.StatusNotFound` is treated as "old cluster, endpoint missing" and returns `(nil, false, nil)`. An unready cluster behind a proxy may also return 404. This silently skips the skip-disk configuration on a cluster that simply hasn't started the endpoint yet.  
**Fix:** Document and confirm the exact error code the Harvester API returns when the cluster is not ready vs. the endpoint not existing, then differentiate the two paths — or retry on 404 up to a short limit before treating it as "endpoint absent."

---

## 6. `installprofile` handler uses `== "true"` while the mutator uses `strconv.ParseBool`

**File:** `pkg/api/installprofile/handler.go:17`  
**Problem:** The handler accepts only literal `"true"`. The VMI mutator uses `strconv.ParseBool`, which also accepts `"TRUE"`, `"1"`, `"t"`. A setting value of `"1"` makes the mutator default images to CDI while the join endpoint reports `false`, so joining nodes still create the default disk.  
**Fix:** Replace `== "true"` with `strconv.ParseBool(settings.SkipLonghornDefaultDisk.Get())`.

---

## 7. No validator for the `skip-longhorn-default-disk` setting

**File:** `pkg/settings/settings.go:76`  
**Problem:** Any string is accepted (e.g. `"yes"`, `"1"`). The image mutator then fails at runtime with a `strconv.ParseBool` error on VMI creation. Changing the value after first-node install (e.g. `false` → `true` on a running cluster) would corrupt disk layout across nodes.  
**Fix:** Add a create/update validator that calls `strconv.ParseBool` and rejects non-bool values. Add documentation warning that the value must not be changed after cluster initialization.

---

## 8. `vmstate-persistence` StorageClass is deleted when skipping Longhorn SCs

**File:** `deploy/charts/harvester/templates/harvester-storageclass.yaml:1`  
**Problem:** The `{{- if and .Values.longhorn.enabled (not .Values.storageClass.skipLonghornStorageClasses) -}}` guard wraps `vmstate-persistence`. KubeVirt references `vmStateStorageClass: "vmstate-persistence"` in `values.yaml:100`. VMs with persisted TPM/EFI state will have their PVCs stuck `Pending`.  
**Fix:** Either keep creating `vmstate-persistence` unconditionally when Longhorn is enabled, or update `kubevirt.spec.configuration.vmStateStorageClass` to point at an existing class (e.g. `longhorn-static`) when skipping.

---

## 9. Update mutator does not fill `spec.backend` from the existing VMI

**File:** `deploy/charts/harvester-crd/templates/harvesterhci.io_virtualmachineimages.yaml:59` + `pkg/webhook/resources/virtualmachineimage/mutator.go`  
**Problem:** The CRD default `spec.backend: backingimage` was removed. On **update**, if `spec.backend` is omitted, the mutator does not backfill from the stored object, and `m.mutators[""]` is nil — the request either panics or is rejected.  
**Fix:** In the update path of the VMI mutator, when `vmi.Spec.Backend` is empty, copy it from `oldVMI.Spec.Backend` before dispatching.

---

## 10. VMI webhook may reject requests before the Setting cache syncs

**File:** `pkg/webhook/resources/virtualmachineimage/mutator.go:94`  
**Problem:** `defaultBackend()` calls `m.settingCache.Get(...)`. If the cache has not synced yet when the first VMI create arrives, it returns `NotFound` and the request is denied. No cache-sync gate exists.  
**Fix:** Add a `cache.WaitForCacheSync` call (or a readiness check) before the webhook starts serving mutation requests, so `SettingCache` is populated before the first admission.

---

## 11. `storageClasses` / `storageClassCache` fields in setting handler are unused

**File:** `pkg/controller/master/setting/handler.go:98`  
**Problem:** Both fields are declared but never referenced — dead code.  
**Fix:** Delete both fields and their corresponding wiring in `Register`.

---

## 12. Disk-size validation mismatch in skip mode (180–249 GiB installs fail)

**File:** `pkg/installer/console/install_panels.go` + `pkg/installer/console/validator.go:46`  
**Problem:** `diskConfirm` checks against `MultipleDiskMinSizeGiB` (180 GiB); `validateAllDiskSizes` and `validator.go` check against `SingleDiskMinSizeGiB` (250 GiB). A disk of 180–249 GiB passes the first check and fails the second.  
**Fix:** Define a `SkipModeDiskMinSizeGiB` constant and use it consistently in `diskConfirm`, `validateAllDiskSizes`, and the matching check in `validator.go`.

---

## 13. `diskCount > 1` guard hides the skip-disk option on single-disk installs

**File:** `pkg/installer/console/install_panels.go:290`  
**Problem:** `if diskCount > 1 && profile == nil` hides the skip option for single-disk machines in create mode. The feature's primary use case is single-disk installs.  
**Fix:** Show the skip selector whenever `profile == nil` (create mode), regardless of `diskCount`. Use `diskCount > 1` only to decide whether to show the separate-data-disk row.

---

## 14. `systemSettings["skip-longhorn-default-disk"]` is ignored during install

**File:** `pkg/installer/config/config.go:92`  
**Problem:** Setting only `systemSettings["skip-longhorn-default-disk"] = "true"` passes validation but `Install.SkipLonghornDefaultDisk` stays `false`. Joining nodes read the cluster setting and skip, creating a split-brain disk layout.  
**Fix:** Before partition generation, derive `Install.SkipLonghornDefaultDisk` from `systemSettings["skip-longhorn-default-disk"]` if the install flag was not set explicitly.

---

## 15. Join token sent over `InsecureSkipVerify` TLS (security)

**File:** `pkg/installer/console/util.go:69`  
**Problem:** `newInsecureProxyClient` sets `InsecureSkipVerify: true`. `applyInstallProfile` sends `Authorization: Bearer <join-token>` through this client. An on-path attacker can present any cert, steal the token, and return arbitrary profile values.  
**Fix:** Pin the server's CA cert (from the join config) and verify it, or document the trust model clearly. Flagged by both CodeRabbit and Vicente-Cheng.

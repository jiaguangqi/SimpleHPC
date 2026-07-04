# Slurm noVNC Desktop Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Run authenticated noVNC desktops as Slurm jobs under the submitting Linux user and expose them through the simpleHPC gateway and VNC desktop list.

**Architecture:** Slurm remains the sole scheduler and lifecycle owner. A published built-in template starts TigerVNC and websockify on the allocated compute node, registers the endpoint with the control plane, and remains alive until cancellation or walltime. The existing authenticated reverse proxy provides browser access without exposing compute ports.

**Tech Stack:** Go, PostgreSQL, Slurm, TigerVNC, websockify, noVNC, static JavaScript UI.

---

## Requirements and decisions

- A desktop runs as the same Linux/LDAP username that submitted it.
- Platform administrators without a same-name Linux account cannot silently run a desktop as root.
- Default walltime is `24:00:00`; users may override it in the template submission form.
- The compute job owns VNC and websockify cleanup through shell traps.
- VNC endpoints register with a one-time run token.
- The browser connects only through `/api/v1/job-template-gateway/:token/`.
- The VNC password is derived from the run token and passed to noVNC only inside the authenticated access URL.
- Slurm state is authoritative for pending/running/finished status.

## Alternatives considered

1. **Slurm-owned runtime (selected):** correct HPC placement, accounting, cancellation, and walltime.
2. **SSH from the web service:** matches the reference project but bypasses Slurm accounting and allocation.
3. **Permanent host agent:** useful at larger scale, but unnecessary for the current single control-plane/compute-node deployment.

## Failure handling

- Missing VNC/noVNC/websockify binaries: job exits with a clear stderr message.
- Username missing from NSS: submission is rejected before `sbatch`.
- VNC or WebSocket port not ready: job exits and trap cleanup runs.
- Callback failure: job exits rather than leaving an inaccessible desktop.
- Walltime/cancel: Slurm signals the job and cleanup kills websockify and the VNC display.
- Gateway target unavailable: return `502` without revealing the compute endpoint.

### Task 1: Runtime contract and identity

**Files:**
- Modify: `backend/internal/service/templates.go`
- Test: `backend/internal/service/templates_test.go`

- [ ] Add failing tests for a 24-hour noVNC default walltime and strict submit-user resolution.
- [ ] Export the execution username to the runtime script.
- [ ] Reject noVNC submissions when the platform username has no same-name Linux identity.
- [ ] Run `go test ./internal/service`.

### Task 2: Built-in VNC template

**Files:**
- Create: `backend/internal/service/builtin_vnc.go`
- Modify: `backend/internal/service/service.go`
- Test: `backend/internal/service/builtin_vnc_test.go`

- [ ] Add a tested TigerVNC/websockify script with deterministic ports, readiness checks, callback registration, and trap cleanup.
- [ ] Upgrade the untouched system template to published status and grant it to all users.
- [ ] Preserve templates already edited by an administrator.
- [ ] Run `go test ./internal/service`.

### Task 3: Authenticated noVNC access

**Files:**
- Modify: `backend/internal/service/templates.go`
- Modify: `backend/internal/httpapi/template_gateway.go`
- Test: `backend/internal/service/templates_test.go`

- [ ] Build a noVNC URL containing the gateway WebSocket path and derived eight-character VNC password.
- [ ] Keep owner-or-manager authorization on every proxied request.
- [ ] Verify WebSocket upgrade traffic reaches the registered websockify endpoint.

### Task 4: Submission and desktop UI

**Files:**
- Modify: `js/job-templates.js`
- Modify: `vnc-desktop.html`

- [ ] Show an adjustable walltime input only for noVNC templates, defaulting to 24 hours.
- [ ] Refresh the VNC job list every 10 seconds.
- [ ] Show access only when both Slurm is running and the endpoint is ready.
- [ ] Show pending, starting, ready, ended, and failed states clearly.
- [ ] Keep terminate mapped to Slurm cancellation.

### Task 5: Deployment verification

- [ ] Run all Go and JavaScript tests.
- [ ] Deploy the backend and frontend to `/data/simpleHPC`.
- [ ] Submit a desktop as a real LDAP/Linux user.
- [ ] Verify `squeue` reports that user and the requested walltime.
- [ ] Verify Xvnc and websockify run on the allocated node under the expected identities.
- [ ] Verify the callback marks the run ready and the list shows “访问桌面”.
- [ ] Open noVNC through the authenticated gateway and verify the desktop session user.
- [ ] Cancel the job and verify VNC/websockify processes and ports are cleaned up.

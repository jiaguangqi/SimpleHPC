# Job List Pagination And Actions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add compact server-backed pagination to the job list and connect running-job actions to real Slurm commands.

**Architecture:** PostgreSQL remains the complete searchable history source, with filtering, count, limit, and offset performed by the backend. Current job actions call Slurm through argument-safe client methods, then refresh the PostgreSQL synchronization table and reload the requested page.

**Tech Stack:** Go, Gin, PostgreSQL, Slurm CLI, vanilla JavaScript, Node test runner.

---

### Task 1: Compact Pagination Model

**Files:**
- Create: `js/job-pagination.js`
- Create: `tests/job-pagination.test.js`
- Modify: `job-list.html`

- [ ] Write failing Node tests for first-page and middle-page pagination tokens.
- [ ] Run `node --test tests/job-pagination.test.js` and confirm the module is missing.
- [ ] Implement compact tokens with first five pages, current-page neighbors, last two pages, and ellipses.
- [ ] Add page-size and target-page controls, defaulting to 15 rows.
- [ ] Run the Node tests and syntax checks.

### Task 2: PostgreSQL Job Pagination API

**Files:**
- Modify: `backend/internal/service/service.go`
- Create: `backend/internal/service/jobs_test.go`
- Modify: `backend/internal/httpapi/router.go`

- [ ] Write failing tests for page-size bounds, offset calculation, status normalization, and keyword filtering parameters.
- [ ] Implement a parameterized `QuerySlurmJobs` query returning `items`, `total`, `page`, `pageSize`, and `totalPages`.
- [ ] Update `GET /api/v1/slurm/jobs` to use server-side pagination while retaining `live=1` for direct Slurm inspection.
- [ ] Run `go test ./...`.

### Task 3: Real Slurm Job Actions

**Files:**
- Modify: `backend/internal/integrations/slurm/client.go`
- Modify: `backend/internal/integrations/slurm/client_test.go`
- Modify: `backend/internal/httpapi/router.go`
- Modify: `job-list.html`

- [ ] Write failing tests for accepted and rejected Slurm job IDs.
- [ ] Add `CancelJob`, `SuspendJob`, and `ResumeJob` wrappers using `scancel`, `scontrol suspend`, and `scontrol resume`.
- [ ] Add POST routes for cancel, suspend, and resume, synchronizing PostgreSQL after a successful command.
- [ ] Render details, terminate, and suspend for running jobs; details and resume for suspended jobs; remove redo/retry actions.
- [ ] Run backend and frontend tests.

### Task 4: Deployment Verification

**Files:**
- Deploy backend binary and changed frontend assets to `/data/simpleHPC`.

- [ ] Back up the current remote binary and frontend files.
- [ ] Restart `simplehpc-backend.service`.
- [ ] Verify page 1 and a middle page through the API.
- [ ] Verify invalid job actions are rejected without invoking Slurm.
- [ ] Confirm the deployed HTML and JavaScript contain the new controls and no redo labels.

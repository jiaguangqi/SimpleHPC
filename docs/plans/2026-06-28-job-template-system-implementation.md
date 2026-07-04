# Job Template System Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a database-backed low-code Slurm template system supporting batch, noVNC, and proxied web applications.

**Architecture:** Extend the existing Go monolith with template, grant, request, run, submission, and gateway APIs. Use PostgreSQL as the source of truth and the existing Redis sessions for authorization. Replace the static template page with an API-driven library and form designer.

**Tech Stack:** Go, Gin, PostgreSQL, Redis, Slurm CLI, HTML/CSS/JavaScript.

---

### Task 1: Template schema and validation

**Files:**
- Create: `backend/internal/service/templates.go`
- Create: `backend/internal/service/templates_test.go`
- Modify: `backend/internal/service/service.go`
- Modify: `backend/migrations/001_init.sql`

1. Write failing tests for schema validation, variable validation, shell quoting, visibility, and generated Slurm headers.
2. Run `cd backend && go test ./internal/service -run Template -v` and verify failure.
3. Implement the model, validation, schema creation, and starter-template seeding.
4. Run the focused tests and then `go test ./...`.

### Task 2: Slurm submission

**Files:**
- Modify: `backend/internal/integrations/slurm/client.go`
- Modify: `backend/internal/integrations/slurm/client_test.go`

1. Write failing tests for parsing `sbatch --parsable` output and rejecting invalid Linux usernames.
2. Add a submission method that writes a protected temporary script and executes `runuser -u USER -- sbatch --parsable`.
3. Verify focused and full backend tests.

### Task 3: Template and authorization APIs

**Files:**
- Create: `backend/internal/httpapi/templates.go`
- Modify: `backend/internal/httpapi/router.go`
- Modify: `backend/internal/service/auth.go`

1. Add authenticated session lookup.
2. Add CRUD, publish, grants, requests, approval, import/export, preview, and submit endpoints.
3. Enforce management and ownership checks at the API boundary.
4. Add handler tests and run `go test ./...`.

### Task 4: Low-code template UI

**Files:**
- Modify: `job-templates.html`
- Create: `js/job-templates.js`
- Modify: `css/theme.css`
- Modify: `js/app.js`

1. Remove static cards and the prototype warning.
2. Render published/draft templates from the API.
3. Add centered dialogs for details, access requests, approvals, import/export, and submission.
4. Add a drag-sort form designer with component settings and script editor.
5. Verify management and user views in the browser.

### Task 5: Interactive runtimes

**Files:**
- Create: `backend/internal/service/template_runtime.go`
- Create: `backend/internal/httpapi/template_gateway.go`
- Modify: `backend/internal/httpapi/router.go`

1. Add one-time endpoint registration.
2. Add run ownership checks and reverse proxy support.
3. Add administrator-owned noVNC and web application runtime commands.
4. Verify batch submission and noVNC readiness on the test cluster.

### Task 6: Deployment and acceptance

1. Build the backend with `cd backend && go test ./... && go build ./cmd/server`.
2. Back up changed remote files and the current binary.
3. Deploy frontend and backend to `/data/simpleHPC`.
4. Restart `simplehpc-backend.service`.
5. Verify API health, template CRUD, publish visibility, access workflow, export/import, preview, and batch submission.
6. Verify the page at `http://10.10.38.152:8080/job-templates.html`.


> @ 20260905 11:05

The precise arrangement is:

- VM Nginx: public-facing, usually ports `80` and `443`.
- AUSE web-container Nginx: internal application server, container port `8080`.
- Both run on the same VM.
- The React frontend is stored inside the web container as static files.
- VM Nginx forwards only `/ause-discovery/` traffic to the web container.

## Container network map

```text
VM Nginx
  |
  | HTTP to 127.0.0.1:8090
  v
web:8080
  |
  | API and health requests
  v
api:8080
  |                  |
  | PostgreSQL       | HTTP
  v                  v
postgres:5432    meilisearch:7700
  |
  v
artifact-data volume
```

Internal DNS names are the Compose service names:

| Address | Used for |
|---|---|
| `web:8080` | AUSE frontend server |
| `api:8080` | Go API |
| `postgres:5432` | PostgreSQL |
| `meilisearch:7700` | Search engine |

Only the web service should be reachable from the VM host. API, PostgreSQL, and Meilisearch do not need published host ports.

## Web-container Nginx routing

Assuming the base path is `/ause-discovery/`:

| Incoming path | Result |
|---|---|
| `/ause-discovery` | Redirects to `/ause-discovery/` |
| `/ause-discovery/assets/*` | Serves compiled CSS, JavaScript, fonts, and images |
| `/ause-discovery/api/v1/*` | Proxies to `api:8080` |
| `/ause-discovery/health/*` | Proxies to `api:8080` |
| `/ause-discovery/*` | Serves the React application |
| Unknown React route | Serves `index.html`; React displays Not Found |

The React fallback is important. A request for:

```text
/ause-discovery/projects/123
```

does not correspond to a physical HTML file. Nginx serves `index.html`, then React reads the URL and renders the Project page.

## Frontend browser routes

All routes begin with `/ause-discovery`.

### Public pages

```text
/
 /search
 /projects/:projectId
 /people/:personId
 /about
 /privacy
 /accessibility
 /terms
 /contact
 /admin/login
```

Examples:

```text
/ause-discovery/
/ause-discovery/search?q=robotics
/ause-discovery/projects/018f...
/ause-discovery/people/018f...
```

### Protected administrator pages

```text
/admin
/admin/projects
/admin/projects/new
/admin/projects/:projectId/edit
/admin/people
/admin/people/:personId
/admin/imports
/admin/imports/:batchId
/admin/search
/admin/audit
```

These are browser routes, not API endpoints.

## Operational health endpoints

These sit outside the versioned API prefix:

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/ause-discovery/health/live` | Confirms that the API HTTP server is running |
| `GET` | `/ause-discovery/health/ready` | Checks database schema and writable storage |

The API also exposes versioned equivalents:

```text
GET /ause-discovery/api/v1/health/live
GET /ause-discovery/api/v1/health/ready
```

## Public API endpoints

Base URL:

```text
/ause-discovery/api/v1
```

### Discovery

| Method | Endpoint | Purpose |
|---|---|---|
| `GET` | `/search` | Search and filter published Projects |
| `GET` | `/projects/{project_id}` | Read a published Project |
| `GET` | `/people/{person_id}` | Read a public Person and associated Projects |
| `GET` | `/catalogs` | Read academic and taxonomy catalog values |

Search supports filters including academic year, semester, program, major, course, Person, Student ID, advisor, taxonomy, Artifact availability, sorting, cursor, and limit.

### Public Artifacts

| Method | Endpoint | Purpose |
|---|---|---|
| `GET` | `/artifacts/{artifact_id}/view` | Display supported content inline |
| `GET` | `/artifacts/{artifact_id}/download` | Download the stored file |

Only active Artifacts belonging to published Projects are publicly accessible.

## Administrator authentication API

| Method | Endpoint | Purpose |
|---|---|---|
| `POST` | `/admin/auth/login` | Authenticate and create a session |
| `POST` | `/admin/auth/logout` | Revoke the current session |
| `GET` | `/admin/auth/session` | Read the current administrator session |
| `GET` | `/admin/auth/csrf` | Obtain the CSRF token required for mutations |

The session is stored in an `ause_session` cookie.

## Administrator Project API

| Method | Endpoint | Purpose |
|---|---|---|
| `GET` | `/admin/projects` | List and filter Projects |
| `POST` | `/admin/projects` | Create a Project |
| `GET` | `/admin/projects/{project_id}` | Read the complete administrator record |
| `PUT` | `/admin/projects/{project_id}` | Replace Project metadata |
| `POST` | `/admin/projects/{project_id}/publish` | Publish a Project |
| `POST` | `/admin/projects/{project_id}/delete` | Soft-delete a Project |
| `POST` | `/admin/projects/{project_id}/restore` | Restore a deleted Project |

Updates use revision numbers to prevent one administrator from silently overwriting another administrator’s changes.

## Administrator People API

| Method | Endpoint | Purpose |
|---|---|---|
| `GET` | `/admin/people` | List and filter People |
| `POST` | `/admin/people` | Create a Person |
| `GET` | `/admin/people/{person_id}` | Read a Person |
| `PATCH` | `/admin/people/{person_id}` | Update a Person |

## Administrator Artifact API

| Method | Endpoint | Purpose |
|---|---|---|
| `POST` | `/admin/projects/{project_id}/artifacts` | Upload an Artifact |
| `PATCH` | `/admin/artifacts/{artifact_id}` | Update Artifact metadata |
| `POST` | `/admin/artifacts/{artifact_id}/replace` | Replace the stored file |
| `POST` | `/admin/artifacts/{artifact_id}/delete` | Soft-delete an Artifact |
| `POST` | `/admin/artifacts/{artifact_id}/restore` | Restore an Artifact |

Uploads and replacements use `multipart/form-data`.

## Administrator import API

| Method | Endpoint | Purpose |
|---|---|---|
| `POST` | `/admin/imports` | Upload CSV or XLSX and create a preview |
| `GET` | `/admin/imports/{batch_id}` | Read import status |
| `GET` | `/admin/imports/{batch_id}/rows` | Page through preview rows |
| `PATCH` | `/admin/imports/{batch_id}/rows` | Save row selections and corrections |
| `POST` | `/admin/imports/{batch_id}/commit` | Commit selected valid rows atomically |
| `GET` | `/admin/imports/{batch_id}/result` | Read the immutable commit result |

## Administrator search-maintenance API

| Method | Endpoint | Purpose |
|---|---|---|
| `GET` | `/admin/search/status` | Read Meilisearch and reconciliation status |
| `POST` | `/admin/search/projects/{project_id}/reindex` | Queue one Project for reindexing |
| `POST` | `/admin/search/rebuilds` | Start a full index rebuild |
| `GET` | `/admin/search/rebuilds/{operation_id}` | Read rebuild progress |

## Administrator audit API

| Method | Endpoint | Purpose |
|---|---|---|
| `GET` | `/admin/audit-events` | List audit events with action, actor, and cursor filters |

## Security boundary

Public endpoints require no account.

Administrator reads require the session cookie. Administrator mutations normally require:

```text
Cookie: ause_session=...
X-CSRF-Token: ...
Origin: https://the-public-domain
```

The complete request path is therefore:

```text
Browser
  -> VM Nginx
  -> web-container Nginx
  -> Go API
  -> PostgreSQL, Meilisearch, or Artifact storage
```

The `migrate` container and `ausectl` commands expose no HTTP endpoints. They are temporary command-line jobs using the same internal PostgreSQL and Meilisearch connections.
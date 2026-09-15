# Senior Project Discovery Portal
## Comprehensive MVP Product and Technical Specification

**Status:** Historical MVP scope record. The MVP was completed 2026-09-05 at
commit `1cc5dc6` and deployed at `https://life.au.edu/ause-discovery/`. This
document no longer governs current work: active scope lives in
`../implementation-status/backlog.md` and built behavior in
`../implementation-status/completed.md`.  
**Version:** 0.1, archived unchanged  
**Primary audience:** Developer, faculty stakeholders, university administrators, project reviewers  
**Institutional context:** Assumption University of Thailand, Vincent Mary School of Science, Engineering and Technology  
**Purpose:** Define the MVP product scope, behavior, architecture, data model, search model, administration workflow, security baseline, deployment expectations, and future extension points for a searchable institutional archive of senior projects.

---

# 1. Executive Summary

The Senior Project Discovery Portal is a web application for discovering and accessing historical senior projects produced by students of the faculty.

Its primary purpose is to make previous work easy to find and inspect so that students, lecturers, and advisors can use it as a reference source for project inspiration, prior-art awareness, related work, historical context, advisor history, student project history, technology usage, academic browsing, and access to artifacts such as reports, presentation slides, source archives, datasets, and other submitted materials.

The MVP is intentionally a **search and browsing product**. It is not a recommendation platform, social network, project management system, learning management system, research analytics platform, AI extraction pipeline, or student submission portal.

The principal public flow is:

1. A visitor opens the portal.
2. The visitor searches using text, filters, or both.
3. Project-level results are returned.
4. The visitor opens a project.
5. The visitor reviews curated metadata.
6. The visitor views or downloads available artifacts.

The principal administrative flow is:

1. An administrator signs in.
2. The administrator creates or imports project data.
3. Data is validated and previewed.
4. Valid data is committed to PostgreSQL.
5. A denormalized search document is synchronized to Meilisearch.
6. Administrators can later correct metadata, manage artifacts, archive/delete records (delete doesn't actually delete, it gets archived), and rebuild search state if needed.

The MVP is designed for deployment on a university-managed virtual machine using Docker Compose, Nginx, React, Go, PostgreSQL, and Meilisearch.

---

# 2. Product Definition

## 2.1 Working Name

**Senior Project Discovery Portal**

Alternative descriptive names may include:

- Senior Project Archive
- Senior Project Repository
- Senior Project Index
- Senior Project Discovery System
- Faculty Senior Project Portal

The final public product name is not fixed by this document.

## 2.2 Product Category

The system is an:

**Institutional searchable senior-project repository with faceted discovery.**

## 2.3 Core Problem

Historical senior projects may exist as folders, reports, slide decks, source archives, or records distributed across storage systems. Without a unified index, users cannot efficiently answer questions such as:

- What projects have been done in computer vision?
- What projects used React?
- What projects were completed in a particular academic year or semester?
- Which projects were advised by a particular lecturer?
- What projects were completed by a specific student?
- What projects exist in education, transportation, healthcare, security, finance, or another domain?
- Which projects have source code, slides, reports, or datasets?
- What related work already exists that a student could learn from or build upon?

The portal solves the discovery problem by indexing curated project metadata and connecting each project to its artifacts.

---

# 3. Product Goals

## 3.1 Primary MVP Goals

The MVP shall:

- provide fast text search over curated metadata;
- provide faceted filtering and browsing;
- make projects discoverable by title, abstract, people, academic metadata, and controlled classification;
- provide public project detail pages;
- provide public person detail pages;
- provide access to project artifacts;
- support PDF viewing through the browser;
- support artifact downloads;
- support local administrator authentication;
- provide admin project creation and editing;
- support manual single-project entry;
- support CSV bulk import;
- support XLSX bulk import;
- validate imports before commit;
- provide import preview and error reporting;
- support controlled archival/deletion;
- support project-level search reindexing;
- support full search-index rebuilding;
- support configurable non-root deployment paths;
- be deployable using Docker Compose on a university VM;
- be ready for future localization and dark mode without large refactors;
- preserve a clear distinction between canonical data and derived search data;
- remain maintainable by a single primary developer.

## 3.2 Secondary Goals

The MVP should:

- use conventional, well-supported technologies;
- avoid infrastructure that is unnecessary at the expected scale;
- keep realistically replaceable external integrations behind small boundaries;
- support future Microsoft Entra ID integration;
- support future organization-only access if approved;
- support future analytics and telemetry;
- support future backup and disaster-recovery workflows;
- support future search improvements without changing the core project model.

---

# 4. Explicit MVP Non-Goals

The following are not MVP requirements:

- recommendation algorithms;
- personalized recommendations;
- similar-project algorithms;
- semantic or vector search;
- report-body full-text search;
- PowerPoint preview;
- custom PDF viewer;
- automatic PDF metadata extraction;
- AI tagging inside this application;
- AI document processing inside this application;
- Microsoft Entra ID authentication;
- organization-only access;
- student accounts;
- lecturer accounts;
- user-editable student profiles;
- user-editable lecturer profiles;
- popularity ranking;
- trending projects;
- page-view analytics;
- search analytics;
- source-code browsing;
- source-code search;
- document annotation;
- social features;
- comments;
- likes;
- ratings;
- messaging;
- student submissions;
- Redis;
- Kafka;
- RabbitMQ;
- distributed workers;
- microservices;
- automatic backups;
- automated disaster recovery;
- PowerPoint-to-PDF conversion;
- embedded Office viewers.

These may be considered in later stages where explicitly described in the roadmap.

---

# 5. User Types and Access

## 5.1 Public Visitor

A public visitor can:

- view the homepage;
- search projects;
- apply filters;
- browse results;
- view project pages;
- view person pages;
- inspect project metadata;
- view PDF artifacts in a new browser tab;
- download allowed artifacts.

No public account is required in the MVP.

## 5.2 Administrator

An administrator can perform all public actions and additionally:

- sign in using local credentials;
- create projects;
- edit projects;
- publish projects;
- archive/delete projects;
- manage people and project-person relationships;
- manage project taxonomy assignments;
- upload artifacts;
- remove or replace artifacts;
- perform manual project entry;
- upload CSV imports;
- upload XLSX imports;
- preview imports;
- inspect import warnings/errors;
- confirm imports;
- reindex a project;
- rebuild the full search index;
- access relevant operational and audit information.

The MVP may expose one effective administrator role even if the internal authorization model anticipates future expansion.

For the MVP, admin account crud will be with operator-side CLI command.

## 5.3 Future Roles

Potential later roles include:

- university member;
- student;
- lecturer;
- advisor;
- editor;
- administrator;
- system administrator.

These should not complicate the MVP implementation prematurely.

---

# 6. Core User Journeys

## 6.1 Search by Topic

1. Visitor enters a phrase such as `computer vision`.
2. The system searches curated project metadata.
3. Matching projects are ranked.
4. Visitor optionally applies filters.
5. Visitor opens a project result.
6. Visitor reviews metadata and artifacts.

## 6.2 Search by Student

1. Visitor searches a student name or student identifier.
2. Matching projects with associated people are returned.
3. Visitor opens the person page.
4. The page lists that person's associated projects.

## 6.3 Search by Advisor

1. Visitor searches an advisor name.
2. Results show projects associated with that advisor.
3. Visitor may open the advisor's person page.
4. The page lists advised projects and optionally committee participation.

## 6.4 Browse by Facet

A visitor may browse by combinations such as:

- year;
- semester;
- program;
- advisor;
- category;
- platform;
- domain;
- topic;
- technology.

Text search and filters may be used together.

## 6.5 Access Artifacts

On a project page:

- PDF: `View` and `Download`;
- PPTX: `Download` only;
- ZIP/source archive: `Download` only;
- other non-previewable artifacts: `Download`.

No custom document viewer is required.

## 6.6 Admin Creates One Project

1. Admin signs in.
2. Admin opens project creation.
3. Admin enters metadata.
4. Admin associates people.
5. Admin assigns controlled taxonomy.
6. Admin uploads artifacts.
7. Validation runs.
8. Admin saves/publishes.
9. PostgreSQL commits canonical data.
10. Search document is indexed in Meilisearch.

## 6.7 Admin Bulk Imports Projects

1. Admin uploads CSV/XLSX.
2. Adapter parses rows into canonical import drafts.
3. Data is normalized.
4. Data is validated.
5. Preview shows valid rows, warnings, errors, duplicate candidates, and proposed normalized values.
6. Admin confirms.
7. Valid records are committed to PostgreSQL.
8. Search documents are generated.
9. Import summary is shown.

---

# 7. Search and Discovery

## 7.1 Search Philosophy

Search is the primary product capability.

The MVP should optimize for:

- predictable relevance;
- useful metadata matching;
- easy filtering;
- readable results;
- low operational complexity.

The MVP should not attempt to infer intent through recommendation or semantic systems.

## 7.2 Search Engine

The MVP shall use **Meilisearch**.

Meilisearch is a separate service that maintains a derived search index.

## 7.3 Canonical Data Ownership

**PostgreSQL is authoritative.**

Meilisearch is disposable derived state.

The entire search index must be rebuildable from PostgreSQL.

## 7.4 Searchable Fields

The MVP search index should include, where available:

- project title;
- title aliases;
- project reference code;
- abstract;
- student names;
- student identifiers;
- advisor names;
- co-advisor names;
- committee names if included;
- academic year;
- semester;
- program;
- major where applicable;
- course code;
- category labels;
- platform labels;
- domain labels;
- topic labels;
- technology labels.

## 7.5 Explicitly Non-Searchable Content

The MVP shall not search:

- report body text;
- PowerPoint contents;
- source-code contents;
- ZIP contents;
- arbitrary attachment contents;
- literature-review text;
- citation/reference sections.

Report-body search may be evaluated in a later stage after stakeholder discussion.

## 7.6 Ranking Intent

Exact weights are implementation-time tuning, but intended priority is roughly:

1. exact student/reference identifiers;
2. project title;
3. title aliases;
4. student names;
5. advisor names;
6. controlled taxonomy labels;
7. abstract;
8. other metadata.

Typo tolerance should be available for human-readable text fields.

Exact identifiers should use stricter matching.

## 7.7 Faceted Filters

The system should support filters in these groups.

### Academic
- year;
- semester;
- program;
- major;
- course.

### People
- advisor;
- student, where useful.

### Project Classification
- category;
- platform;
- domain;
- topic;
- technology.

### Artifact Availability
Potentially:
- has report;
- has slides;
- has source code;
- has dataset;
- has selected artifact type.

Artifact-availability filtering is desirable but can be lower priority than the primary academic and project facets if implementation time becomes constrained.

## 7.8 Multi-Value Semantics

Several classifications are intentionally multi-valued.

A project may be both:

- `AI / Data`;
- `Software / Application`.

A project may have multiple:

- technologies;
- topics;
- domains;
- platforms;
- advisors;
- students.

The database and search index must not assume these are exclusive single-value enums.

## 7.9 Search Result Unit

One search result represents one **Project**.

Artifacts and PDF pages are never top-level MVP search results.

## 7.10 Search Result Content

A search result should be able to show:

- project title;
- year;
- semester;
- concise abstract/excerpt;
- student names;
- advisor names;
- selected classifications;
- available artifact indicators;
- reference code where useful.

Search results should favor information density and fast scanning.

---

# 8. Homepage

## 8.1 Search-First Design

The homepage must work with no dynamic content beyond search/navigation.

A valid homepage may consist primarily of:

- institutional/product branding;
- central search field;
- selected filter/browse shortcuts;
- navigation.

## 8.2 Optional Featured Projects

A manually configured list of project identifiers may be shown as a Featured Projects section.

The list may live in application configuration.

Requirements:

- no popularity model is required;
- no database `featured` boolean is required;
- the page works if the configured list is empty;
- invalid/deleted project references are ignored gracefully.

## 8.3 Excluded Homepage Features

The MVP homepage shall not include algorithmic:

- most viewed;
- trending;
- recommended;
- personalized;
- popular.

---

# 9. Project Identity and Core Data Model

## 9.1 Canonical Project Identity

Each project shall have:

- `id`: UUID generated/owned by the application;
- `reference_code`: optional legacy/external university reference string.

The UUID is canonical.

The reference code is informational/searchable reference data.

## 9.2 Reference Code Rules

The system must not invent undocumented semantics for legacy codes.

Reference codes:

- are stored as strings;
- do not determine UUID identity;
- are searchable;
- should not be parsed to infer year/course/program unless semantics are officially documented;
- should not receive a uniqueness constraint unless the university confirms uniqueness.

## 9.3 Core Project Fields

The conceptual project model should support:

- UUID;
- reference code;
- canonical title;
- title aliases;
- abstract;
- academic year;
- semester;
- program;
- major where applicable;
- course;
- project status;
- created timestamp;
- updated timestamp;
- archived/deleted timestamp where applicable;
- optional extension metadata.

## 9.4 Project Lifecycle

A minimal lifecycle may include:

- `draft`;
- `published`;
- `archived` or `deleted`.

Exact naming may be finalized later.

Published projects are publicly visible.

Draft projects are administrator-only.

Archived/deleted projects are excluded from normal public browsing/search.

## 9.5 Extension Metadata

A controlled JSONB field such as `extra_metadata` may be used for genuinely irregular future fields.

JSONB must not replace relational modeling for values that are frequently:

- searched;
- filtered;
- validated;
- displayed;
- related to other entities.

---

# 10. People and Project Relationships

## 10.1 Person Entity

Students, lecturers, advisors, and committee members should be modeled as people.

A conceptual Person may contain:

- UUID;
- display name;
- normalized name;
- student/staff/institutional identifier where applicable;
- person type where useful;
- created timestamp;
- updated timestamp.

## 10.2 Project-Person Relationship

Participation shall use a relationship entity rather than embedding names directly into the project record.

Conceptual roles include:

- student/member;
- advisor;
- co-advisor;
- committee member.

## 10.3 Person Pages

A public person page may show:

- display name;
- student id;
- associated projects;
- role in each project;
- advisor history;
- committee history.

A Person does not imply a login account.

## 10.4 Authentication Identity Separation

Future Microsoft Entra identity must remain separate from academic Person records.

An authenticated identity may later be linked to a Person record, but the system must not assume every Person has an account.

---

# 11. Academic Metadata

Academic metadata should be modeled as structured fields/entities rather than generic tags.

Potential values include:

- academic year;
- semester;
- program;
- major;
- course;
- faculty/school.

Historical project metadata should remain historically accurate even if curriculum or program names change later.

The system should not assume a person's current academic information is identical to the information associated with a historical project.

---

# 12. Controlled Taxonomy

## 12.1 Principles

The MVP shall use a small controlled taxonomy that is:

- understandable;
- searchable;
- filterable;
- multi-valued where appropriate;
- developer-maintainable;
- refactorable without rewriting project records.

## 12.2 Stable Key vs Display Label

Taxonomy values shall have a stable internal key separate from localized human labels.

Example:

```yaml
key: software_application
label:
  en: Software / Application
```

Project data references the stable key.

Changing the label from `Software / Application` to `Application` must not require rewriting project records.

## 12.3 Initial Taxonomy Dimensions

### Category

Broad nature of the work. Initial candidates include:

- Software / Application;
- AI / Data;
- Research;
- Game;
- Hardware / IoT.

Projects may have multiple categories.

### Platform

Initial candidates include:

- Web;
- Mobile;
- Desktop;
- Embedded / Hardware;
- Game;
- Other.

### Domain

Initial candidates might include:

- Education;
- Business;
- E-commerce;
- Healthcare;
- Transportation;
- Entertainment;
- Security;
- Finance;
- Campus;
- Other.

The exact vocabulary is intentionally not fixed.

### Topic

A small controlled group for useful concepts such as:

- Computer Vision;
- OCR;
- NLP;
- Gamification;
- Geolocation.

The exact list remains configurable.

### Technology

Examples:

- React;
- Go;
- PostgreSQL;
- OpenCV;
- Unity;
- Python;
- Firebase.

Only technologies actually used by the project should be treated as used technologies.

## 12.4 Taxonomy Administration

MVP taxonomy definitions are developer-controlled.

A full administrator taxonomy CMS is not required.

Taxonomy definitions should be centralized enough to allow easy developer changes.

## 12.5 Taxonomy Changes

Changing a display label is not a project-data migration.

Changing the meaning of a stable key, merging concepts, or splitting one concept into several is a real taxonomy migration and may require explicit data migration.

---

# 13. Artifact Model

## 13.1 Flexible Child Records

Artifacts shall be independent child records of Project.

The project table must not use fixed columns such as:

- `report_url`;
- `slides_url`;
- `code_url`.

## 13.2 Artifact Fields

A conceptual Artifact may include:

- UUID;
- project UUID;
- artifact type;
- display name;
- original filename;
- internal storage key;
- MIME type;
- file size;
- visibility;
- created timestamp;
- updated timestamp;
- archived/deleted timestamp if needed.

## 13.3 Initial Artifact Types

Candidates include:

- report;
- slides;
- source code;
- proposal;
- poster;
- dataset;
- demo video;
- external link;
- other.

The list may evolve.

## 13.4 PDF Behavior

PDF artifacts shall support:

- `View`;
- `Download`.

`View` opens the PDF in a new browser tab using the browser's native PDF capability.

The MVP shall not include a custom PDF viewer.

## 13.5 PowerPoint Behavior

PPTX artifacts support:

- `Download` only.

The MVP shall not:

- preview PowerPoint;
- convert PowerPoint;
- embed Office viewers.

## 13.6 Other Files

ZIP/source archives and other non-previewable files support download.

No source-code browsing is required.

---

# 14. Artifact Storage

## 14.1 Storage Backend

MVP artifacts should be stored on the university VM filesystem using a persistent Docker volume or host-mounted data directory.

Large artifact bytes should not be stored directly inside PostgreSQL.

## 14.2 Storage Keys

Physical storage names must not be derived directly from uploaded filenames.

Each file receives an application-generated storage identifier/key.

Example:

- original filename: `Final Report.pdf`;
- storage key: generated UUID/random identifier.

## 14.3 Controlled Access

Artifacts should be accessed through application-defined routes or controlled file-serving logic.

Conceptual routes:

```text
GET /api/artifacts/{artifactId}/view
GET /api/artifacts/{artifactId}/download
```

Published artifacts are public in the MVP.

The architecture should permit future authorization checks without changing file identity.

## 14.4 File Safety

The backend must:

- enforce upload size limits;
- validate expected file/content type reasonably;
- generate safe storage names;
- prevent path traversal;
- never execute uploaded code/files;
- prevent arbitrary VM filesystem access;
- restrict filesystem permissions to the required artifact area.

---

# 15. Data Entry and Import

## 15.1 Philosophy

All input methods should converge on one canonical validation pipeline.

Input sources are adapters.

## 15.2 MVP Input Methods

The MVP shall support:

- manual single-project entry;
- CSV bulk import;
- XLSX bulk import.

## 15.3 Canonical Import Draft

All adapters should produce a common internal representation conceptually similar to:

```text
ProjectImportDraft
  title
  reference_code
  abstract
  academic metadata
  students[]
  advisors[]
  committee[]
  categories[]
  platforms[]
  domains[]
  topics[]
  technologies[]
  artifacts[]
```

The exact shape is an implementation detail.

## 15.4 External AI Extraction

Automatic PDF extraction and AI tagging are outside this application.

External tools may prepare data for import.

The portal should not care whether import data came from:

- a human;
- a script;
- an AI tool;
- an existing university system.

It only cares that the input conforms to a supported import format.

## 15.5 Import Validation

Before commit, imported records should be checked for:

- required fields;
- malformed values;
- invalid taxonomy keys;
- invalid academic metadata;
- duplicate candidates;
- person ambiguity where applicable;
- invalid reference formatting;
- artifact metadata problems where applicable.

## 15.6 Import Preview

Bulk import must have a preview step showing:

- valid records;
- warnings;
- errors;
- skipped records;
- possible duplicates;
- normalized values.

The admin must explicitly confirm before persistence.

## 15.7 Exact CSV/XLSX Schema

Exact columns, sheet names, data encodings, and template format are intentionally deferred to implementation specification.

---

# 16. Administrator Interface

## 16.1 Administration Areas

The admin interface should cover:

- projects;
- people;
- artifacts;
- imports;
- search-index operations;
- authentication/session functions;
- audit/operational information where implemented.

## 16.2 Project Management

Admin must be able to:

- create;
- edit;
- save draft;
- publish;
- archive/delete;
- assign people;
- assign taxonomy;
- add/remove artifacts;
- update title/abstract/reference metadata.

## 16.3 Person Management

Admin should be able to:

- create person records;
- edit person records;
- associate people with projects;
- resolve obvious duplicate-person issues.

Advanced merge tooling may be deferred.

## 16.4 Search Maintenance

Admin must have a mechanism to:

- reindex one project;
- rebuild the entire search index.

This may be exposed through UI or protected operational tooling.

---

# 17. Deletion and Destructive Actions

## 17.1 Philosophy

Senior projects are archival records intended to remain available.

Deletion primarily exists for:

- incorrect uploads;
- duplicate records;
- mistaken submissions;
- exceptional administrative corrections.

## 17.2 Confirmation Friction

Project deletion must require deliberate confirmation.

Preferred interaction:

- administrator types the project's reference code; or
- administrator types another project-specific confirmation value.

Password re-entry is not required.

## 17.3 Soft Deletion

Normal admin deletion should not immediately destroy database records and artifact bytes.

The project should be hidden from public browsing/search and marked with deletion/archive metadata.

## 17.4 Search Removal

Archived/deleted projects must be removed from public Meilisearch results.

## 17.5 Permanent Destruction

Immediate permanent physical deletion is not an MVP requirement.

Permanent cleanup may be added later as a more privileged operation.

## 17.6 Bulk Destruction

The MVP must not provide a casual "delete everything" operation.

## 17.7 Threat Model Note

Confirmation dialogs prevent mistakes but do not stop an attacker who already controls an administrator session.

Compromise resilience depends on:

- authentication;
- authorization;
- audit logging;
- delayed physical destruction;
- secure filesystem permissions;
- future independent backups.

---

# 18. Local Administrator Authentication

## 18.1 Scope

Only administrators authenticate in the MVP.

## 18.2 Conceptual Account Model

```text
users
local_credentials
sessions
```

User identity should use UUID and remain independent of authentication provider.

## 18.3 Password Storage

Passwords must use a modern password hashing algorithm.

Preferred default: **Argon2id**.

Passwords must never be stored using plaintext, reversible encryption, or raw unsalted cryptographic hashes.

## 18.4 Session Model

Use server-side sessions with cryptographically random opaque session identifiers.

The session identifier should be stored in an HTTP cookie with appropriate settings such as:

- `HttpOnly`;
- `Secure` in HTTPS production;
- suitable `SameSite`;
- explicit expiration.

Authentication credentials/tokens should not be stored in browser localStorage.

## 18.5 CSRF

State-changing authenticated actions must use appropriate CSRF protection.

Cookie attributes should be treated as defense in depth, not the only CSRF mechanism.

## 18.6 Login Protections

Include:

- basic rate limiting;
- secure verification;
- session expiration;
- logout;
- expired-session invalidation;
- relevant auth/security logs.

## 18.7 Authorization

Protected endpoints require explicit authorization checks.

Conceptual permissions may include:

- `project.create`;
- `project.edit`;
- `project.delete`;
- `artifact.upload`;
- `artifact.delete`;
- `import.execute`;
- `search.reindex`.

The MVP may map all permissions to one administrator role.

---

# 19. Future Microsoft Entra ID

Microsoft Entra ID is a post-MVP stage.

The application user model must remain separate from the authentication provider.

Future conceptual model:

```text
Application User
  ├── Local Credential
  └── Entra Identity
```

Local credentials may later be retained or disabled according to university policy.

Whether project browsing eventually becomes organization-only is intentionally undecided.

The architecture should allow:

- public browsing;
- organization-only browsing;
- mixed access policies.

---

# 20. Search Synchronization

## 20.1 Standard Write Flow

When project searchable metadata changes:

1. validate request;
2. update PostgreSQL;
3. commit transaction;
4. build search document;
5. update Meilisearch.

PostgreSQL success must not depend on Meilisearch availability.

## 20.2 Search Update Failure

If PostgreSQL succeeds and Meilisearch fails:

- canonical data stays committed;
- failure is logged;
- project is eligible for retry/reindex;
- administrator/operations can rebuild consistency.

## 20.3 Reindex Operations

The system shall support:

### Reindex Project
Regenerate one project's search document from PostgreSQL.

### Rebuild Entire Index
Read all eligible published project metadata from PostgreSQL and regenerate Meilisearch.

## 20.4 No Queue Infrastructure

MVP shall not require:

- Kafka;
- RabbitMQ;
- Redis queues;
- distributed workers.

A database-backed outbox can be considered later only if real operational experience justifies it.

---

# 21. Search Document Model

Meilisearch contains a denormalized representation containing only information needed for:

- search;
- filters;
- ranking;
- result rendering.

Example:

```json
{
  "id": "project-uuid",
  "reference_code": "26037",
  "title": "Game Realism through Graphics and Physics",
  "title_aliases": [],
  "abstract": "...",
  "year": 2026,
  "semester": 1,
  "students": ["..."],
  "student_ids": ["..."],
  "advisors": ["..."],
  "programs": ["..."],
  "categories": ["game"],
  "platforms": ["game"],
  "domains": ["entertainment"],
  "topics": ["physics_simulation"],
  "technologies": ["unreal_engine"]
}
```

Administrative notes, upload internals, audit data, and unrelated database fields should not be copied to the search index.

---

# 22. Backend Architecture

## 22.1 Language

**Go**

## 22.2 Architectural Style

**Modular monolith**

No microservices are required.

## 22.3 Suggested Feature-Oriented Package Structure

```text
internal/
  projects/
  people/
  artifacts/
  taxonomy/
  search/
  imports/
  admin/
  auth/
  database/
  http/
```

Exact naming is not fixed.

## 22.4 Practical Abstraction

Interfaces should exist where replacement is realistically plausible.

Good candidates:

- search engine;
- artifact storage;
- authentication provider;
- import adapters.

Avoid interfaces and architecture layers that exist only for ceremony.

## 22.5 Search Boundary

Conceptually:

```go
type SearchEngine interface {
    Search(...)
    IndexProject(...)
    RemoveProject(...)
    Rebuild(...)
}
```

MVP implementation uses Meilisearch.

## 22.6 Artifact Storage Boundary

The MVP uses local filesystem storage, but the application structure should not require project/business logic to know raw filesystem layout.

---

# 23. Frontend Architecture

## 23.1 Framework

**React**

Use plain React, not Next.js.

## 23.2 Build Tool

**Vite**

## 23.3 Routing

Client-side routing must support a configurable base path.

The application must not assume deployment at `/`.

## 23.4 API Configuration

Frontend API URLs must not be hardcoded to the domain root.

API base/path must be environment/configuration aware.

## 23.5 General Frontend Principles

Use:

- reusable components;
- semantic design tokens;
- localization wrappers;
- clear feature-based screens;
- predictable data fetching;
- minimal unnecessary global state.

Exact state-management/data-fetching libraries are deferred.

---

# 24. Non-Root Deployment Requirement

The application must work under deployments such as:

```text
https://example.edu/
```

and:

```text
https://example.edu/senior-projects/
```

The configured base path must be respected by:

- Vite assets;
- React routes;
- links;
- redirects;
- auth redirects;
- API paths;
- artifact links where relevant.

Production Nginx must provide SPA history fallback so refreshing a nested route works.

---

# 25. Localization Readiness

## 25.1 MVP Language

English may be the only implemented language at launch.

## 25.2 Requirement

User-interface strings should be routed through a mature localization library such as `react-i18next`.

Avoid scattering hardcoded display text through components.

## 25.3 Future Languages

Potential future support:

- Thai;
- Burmese;
- Chinese;
- other languages as needed.

## 25.4 Same Interface

Localization changes text, not the fundamental UI.

## 25.5 Project Content

Titles, abstracts, names, reports, and artifacts are not automatically translated.

Official localized project metadata may be added later if supplied.

---

# 26. Theme and Dark Mode Readiness

The MVP uses a light theme.

Colors should be defined using semantic design tokens such as:

- background;
- surface;
- text;
- muted text;
- border;
- primary;
- secondary;
- accent;
- destructive.

Dark mode is future scope.

The architecture should allow dark mode by replacing theme tokens rather than rewriting components.

---

# 27. Visual Design Direction

The UI should feel:

- professional;
- academic;
- editorial;
- institutional;
- modern;
- restrained;
- information-focused.

Avoid:

- heavy glassmorphism;
- generic blue/purple SaaS gradients;
- excessive rounded cards;
- heavy shadows;
- stereotypical AI-product visuals;
- decorative effects that reduce readability.

Preferred traits:

- white/light background;
- thin rules;
- strong typography;
- clear hierarchy;
- mostly square or lightly softened geometry;
- limited shadow;
- angled/parallelogram motifs where appropriate;
- restrained institutional colors.

Known desired faculty/university color family:

- red;
- purple;
- white;
- yellow as a limited secondary/accent.

Exact approved color values and logos will be added later.

The portal does not need to imitate the current university website style.

---

# 28. Main Screens

## 28.1 Public

- Home;
- Search/Browse;
- Project Detail;
- Person Detail;
- Not Found;
- About;
- Privacy Policy;
- Terms of Use or equivalent;
- Accessibility;
- Contact/Institutional Information.

## 28.2 Admin

- Login;
- Admin Home;
- Project List;
- Create Project;
- Edit Project;
- Delete/Archive Confirmation;
- People List;
- Create/Edit Person;
- Artifact Management;
- Import Upload;
- Import Preview;
- Import Result;
- Search Reindex/Rebuild.

Exact navigation may evolve during implementation.

---

# 29. Project Detail Page

A project page should be capable of displaying:

- title;
- reference code;
- academic year;
- semester;
- program;
- major where applicable;
- course;
- abstract;
- categories;
- platforms;
- domains;
- topics;
- technologies;
- students;
- advisors;
- co-advisors;
- committee;
- artifact list.

People should link to person pages where possible.

Artifact rows should show:

- display name;
- type;
- file format;
- size where useful;
- View if supported;
- Download.

---

# 30. Person Detail Page

A person page should be able to display:

- display name;
- appropriate institutional identifier;
- associated projects;
- role in each project;
- year/semester context.

For faculty, it may surface:

- advised projects;
- committee projects.

A public Person record is not an authenticated account.

---

# 31. Footer, Credits, and Legal Pages

## 31.1 Footer

The footer should support:

- university/faculty identity;
- About;
- Privacy Policy;
- Terms of Use or equivalent;
- Accessibility;
- Contact;
- developer credit.

## 31.2 Developer Credit

The footer may include configurable content such as:

```text
Developed by [Developer Name]
GitHub
```

Developer name and GitHub URL should be configuration/content values rather than duplicated component strings.

## 31.3 Legal Text

Final legal wording should use approved university content where required.

The MVP specification defines the existence and placement of policy pages, not final legal language.

---

# 32. Privacy

The MVP does not require behavioral analytics.

Data collection should be limited to what is needed for:

- project/person records;
- artifacts;
- administrator authentication;
- sessions;
- operational/security logs.

The system may display names and institutional/student identifiers, subject to university policy.

Avoid unnecessary personal-data collection.

Future telemetry should be designed around aggregation, retention limits, and data minimization.

---

# 33. Future Analytics and Telemetry

Analytics are explicitly future scope.

Potential future insights include:

- most searched phrases;
- zero-result searches;
- commonly selected filters;
- project views;
- artifact opens/downloads;
- category interest;
- technology interest;
- search-to-click conversion;
- trends by year/semester.

Institutional insight is the primary motivation.

Popularity/trending presentation features are separate future choices and are not implied by adding analytics.

---

# 34. Security Requirements

## 34.1 Baseline

The system must follow reasonable production security practices for an institutional application.

## 34.2 Required Controls

At minimum:

- HTTPS in production;
- secure password hashing;
- secure session cookies;
- CSRF protection;
- authentication rate limiting;
- authorization checks;
- server-side validation;
- parameterized SQL;
- safe artifact storage keys;
- path traversal prevention;
- upload size limits;
- reasonable MIME/content validation;
- restrictive filesystem permissions;
- secrets outside source control;
- no Meilisearch admin key in the browser;
- security headers where practical;
- audit logging for destructive actions;
- no execution of uploaded files.

## 34.3 Service Exposure

Meilisearch administrative access must not be exposed directly to the public frontend.

Normal flow:

```text
Browser -> Go API -> Meilisearch
```

PostgreSQL should not be exposed publicly.

## 34.4 Artifact Permissions

The application should only have filesystem permissions needed for its own artifact storage.

An upload endpoint must never permit arbitrary VM path access.

---

# 35. Audit Logging

The MVP should record important admin events, including:

- login success/failure;
- project creation;
- project updates;
- publication;
- archive/delete;
- artifact upload;
- artifact removal;
- bulk import;
- search reindex;
- search rebuild.

An audit record should ideally capture:

- event type;
- actor;
- target;
- timestamp;
- selected metadata.

No enterprise audit platform is required.

---

# 36. Performance and Resource Use

Performance matters, but over-optimization is explicitly not a goal.

Expected project volume is modest, with tens of new projects per semester and likely only thousands of total project records over many years.

Guidelines:

- no Redis in MVP;
- no distributed cache;
- no message queue;
- use browser/HTTP caching;
- use Nginx for static assets;
- use PostgreSQL appropriately indexed;
- use Meilisearch for search;
- avoid loading large artifacts into application memory unnecessarily;
- avoid unbounded logs/temp files.

Because report contents are not indexed, Meilisearch data volume should remain small.

---

# 37. Deployment Architecture

## 37.1 Environment

Target: university-managed raw virtual machine with finite shared RAM and storage.

The application should be reasonably efficient without becoming constrained by premature optimization.

## 37.2 Containerization

Deployment shall use:

**Docker Compose**

## 37.3 Runtime Services

Conceptually:

```text
Nginx
Go API
PostgreSQL
Meilisearch
```

React is built into static assets and served through Nginx.

## 37.4 Container Images

Application images may be pulled from:

- GitHub Container Registry;
- Docker Hub;
- another approved registry.

Production versions should be pinned.

Avoid using an unversioned `latest` tag as the production deployment contract.

## 37.5 Operational Deployment

A simple deployment may resemble:

```bash
docker compose pull
docker compose up -d
```

Exact CI/CD is implementation scope.

---

# 38. Nginx Requirements

The repository should include a straightforward example Nginx configuration.

It should demonstrate:

- React static serving;
- SPA history fallback;
- configurable base-path/subdirectory deployment;
- reverse proxy to Go;
- HTTPS-ready assumptions;
- reasonable upload limits;
- artifact routing as needed.

The example is guidance, not a mandate for the university's exact infrastructure.

---

# 39. Disk and Container Hygiene

Because the VM has finite storage:

- Docker/container logs should rotate or have limits;
- unused old application images should not accumulate indefinitely;
- artifact files should live in an explicit persistent location;
- temporary import files should be removed after processing;
- temporary upload files should be cleaned;
- build artifacts should not accumulate on production;
- PostgreSQL and Meilisearch use persistent volumes;
- artifact data uses persistent volume/host storage.

No aggressive storage optimization is required.

---

# 40. Backup and Disaster Recovery

Backup automation is not part of the MVP.

The architecture should nevertheless make future backup straightforward.

## 40.1 Critical State

Critical state includes:

- PostgreSQL;
- artifact files;
- selected application configuration where appropriate.

Meilisearch is derived and rebuildable.

## 40.2 Future Backup Capability

A future workflow may generate a compressed archive containing:

- a proper PostgreSQL dump;
- artifact files;
- a manifest;
- selected metadata/configuration.

Exact implementation, storage destination, retention, encryption, scheduling, and restore procedure are deferred to a later dedicated design.

## 40.3 Compromise Resilience

Future backups should eventually be stored in a way that compromise of the normal application does not automatically grant the attacker the ability to destroy all recoverable copies.

---

# 41. Future Roadmap

## 41.1 Near-Future Candidates

- Microsoft Entra ID;
- organization-only access if approved;
- richer user roles/permissions;
- backup/export workflow;
- disaster recovery procedures;
- analytics/telemetry;
- additional import adapters;
- integration with university systems.

## 41.2 Search Evolution

Possible future features:

- report full-text search;
- semantic/vector search;
- synonyms;
- query analytics;
- similar projects;
- recommendations;
- search suggestions.

None are required for MVP.

## 41.3 UI Evolution

- Thai localization;
- additional languages;
- dark mode;
- richer faculty-specific functionality;
- additional admin tools.

---

# 42. API Design Principles

The final REST contract belongs in implementation specification, but route design should be predictable.

Potential public groups:

```text
/api/projects
/api/people
/api/search
/api/artifacts
```

Potential admin groups:

```text
/api/admin/auth
/api/admin/projects
/api/admin/people
/api/admin/artifacts
/api/admin/imports
/api/admin/search
```

Conceptual endpoints may include:

```text
GET    /api/projects/{id}
GET    /api/people/{id}
GET    /api/search
GET    /api/artifacts/{id}/view
GET    /api/artifacts/{id}/download

POST   /api/admin/projects
PATCH  /api/admin/projects/{id}
DELETE /api/admin/projects/{id}

POST   /api/admin/imports/preview
POST   /api/admin/imports/commit

POST   /api/admin/search/reindex/{projectId}
POST   /api/admin/search/rebuild
```

Exact paths are not fixed.

---

# 43. Error Handling

Public users should receive understandable errors without internal stack traces.

Admin import errors should contain enough detail to correct data.

Server logs should preserve technical detail.

Important error classes:

- validation error;
- unauthorized;
- forbidden;
- not found;
- conflict;
- import parse error;
- artifact upload error;
- search unavailable;
- internal error.

A temporary Meilisearch outage should not make PostgreSQL-backed project detail pages unavailable.

---

# 44. Observability

Minimum operational visibility should include:

- structured application logs;
- request/error logging;
- startup configuration validation;
- PostgreSQL health check;
- Meilisearch health check;
- container health checks where practical.

Prometheus, Grafana, and similar monitoring stacks are not MVP requirements.

---

# 45. Configuration

Environment-specific values must be configurable.

Examples:

- app base path;
- API base path;
- public application URL;
- database connection;
- Meilisearch URL;
- Meilisearch credential/key;
- artifact storage path;
- session key material;
- cookie settings;
- max upload size;
- developer credit;
- GitHub URL;
- homepage featured project list.

Secrets must not be committed to source control.

---

# 46. Database Design Principles

## 46.1 PostgreSQL

PostgreSQL is the source of truth.

## 46.2 Relational First

Stable concepts should use relational modeling:

- projects;
- people;
- project-person relationships;
- artifacts;
- programs;
- courses;
- taxonomy values;
- project-taxonomy relationships;
- users;
- sessions.

## 46.3 JSONB Escape Hatch

JSONB may hold irregular extension metadata.

It should not replace normal fields for data that is frequently searched, filtered, validated, or related.

## 46.4 Indexing

Normal PostgreSQL indexes should be added for:

- UUID primary keys;
- foreign keys;
- reference codes;
- statuses;
- common academic filters;
- person identifiers;
- useful timestamps.

Exact indexes are implementation detail.

---

# 47. Suggested Conceptual Entities

The final schema may vary, but should cover concepts similar to:

```text
Project
Person
ProjectPerson
Artifact
Program
Course
TaxonomyValue
ProjectTaxonomy
User
LocalCredential
Session
AuditEvent
ImportDraft / ImportPreview
```

A persistent import-job model is optional if imports are synchronous and temporary.

---

# 48. Failure and Recovery Behavior

## 48.1 Meilisearch Failure

If Meilisearch is unavailable:

- canonical reads from PostgreSQL should still work where possible;
- project detail pages should remain available;
- search may return a controlled unavailable state;
- project edits may still commit;
- failed indexing is logged;
- reindex/rebuild restores consistency.

## 48.2 Search Index Loss

Complete Meilisearch data loss must be recoverable through full rebuild from PostgreSQL.

No manual project-data reconstruction should be required.

## 48.3 Artifact Failure

Missing/corrupt artifacts should not make the entire project page fail.

The system should return a controlled artifact error and preserve project metadata access.

---

# 49. Accessibility

The MVP should follow standard accessible web practices:

- semantic HTML;
- keyboard-accessible controls;
- visible focus states;
- form labels;
- reasonable contrast;
- meaningful button/link text;
- no color-only status communication;
- responsive layouts.

Formal accessibility certification is outside scope unless required.

---

# 50. Responsive Design

The public and admin interfaces should support:

- desktop;
- laptop;
- tablet;
- reasonable mobile widths.

Desktop is likely primary, but search and project reading should remain usable on mobile.

---

# 51. Browser Support

Target current evergreen browsers:

- Chrome;
- Edge;
- Firefox;
- Safari where practical.

Internet Explorer is unsupported.

---

# 52. Data Quality and Review

Historical project documents may contain:

- stale templates;
- title conflicts;
- spelling variations;
- legacy naming;
- incomplete metadata;
- conflicting advisor information.

Imported data must therefore be treated as administratively reviewable data.

The MVP import workflow should emphasize:

- preview;
- validation;
- warnings;
- correction before commit.

---

# 53. URL Identity

URLs should ultimately resolve using stable application identity.

Acceptable patterns include:

```text
/projects/{uuid}
```

or a human-friendly route that still resolves to UUID.

Renaming a project must not break database relationships or artifact ownership.

Exact URL format is deferred.

---

# 54. Maintainability Principles

Prefer:

- clear code;
- feature modules;
- mature libraries;
- explicit configuration;
- versioned database migrations;
- small replaceable boundaries;
- stable identifiers;
- simple operational behavior.

Avoid:

- microservices;
- speculative distributed systems;
- event buses without need;
- generic repository abstractions everywhere;
- premature cache infrastructure;
- custom search engines;
- custom authentication protocols;
- custom localization frameworks;
- custom PDF viewers.

---

# 55. Testing Expectations

Automated tests should focus on high-risk behavior.

## 55.1 Backend Priority Tests

- project validation;
- project CRUD;
- auth/session behavior;
- authorization;
- deletion confirmation checks;
- import parsing;
- import validation;
- taxonomy key handling;
- search document construction;
- reindex/rebuild logic;
- artifact path safety;
- artifact authorization;
- storage key generation.

## 55.2 Frontend Priority Tests

- search flow;
- filter behavior;
- admin login;
- project form validation;
- import preview;
- deletion confirmation;
- base-path-safe routing where practical.

## 55.3 Integration Tests

Useful integrations:

- Go + PostgreSQL;
- Go + Meilisearch;
- artifact upload/download;
- search synchronization after project updates.

Exact coverage target is not fixed.

---

# 56. Database Migrations

Database schema changes must use versioned migrations.

Production schema evolution should not rely on ad hoc manual table editing.

The exact migration library/tool is deferred.

---

# 57. Search Schema Evolution

Search configuration may evolve independently of canonical PostgreSQL data.

The system should make it possible to:

- add/remove searchable fields;
- change filterable fields;
- tune ranking;
- add synonyms later;
- rebuild the index.

A search schema change should not require destructive canonical-data modification.

---

# 58. MVP Acceptance Criteria

The MVP can be considered functionally complete when the following are true.

## 58.1 Public Discovery

- Site is accessible without authentication.
- Visitor can search projects.
- Visitor can filter by agreed facets.
- Search returns project-level results.
- Visitor can open a project page.
- Visitor can open a person page.
- Visitor can access artifacts.

## 58.2 Artifact Handling

- PDF View opens browser-native PDF view in a new tab.
- PDF Download downloads the PDF.
- PPTX supports download only.
- ZIP/source supports download.
- Artifact paths cannot escape configured storage.

## 58.3 Administration

- Admin can sign in and out.
- Admin can create a project.
- Admin can edit a project.
- Admin can manage project-person relationships.
- Admin can manage taxonomy assignments.
- Admin can upload artifacts.
- Admin can remove/archive artifacts.
- Admin can archive/delete a project using explicit confirmation.
- Archived/deleted projects disappear from public search.
- Admin can manually create a project.
- Admin can upload CSV.
- Admin can upload XLSX.
- Admin can preview import.
- Import validation shows errors/warnings.
- Admin must confirm before persistence.

## 58.4 Search Synchronization

- New published projects become searchable.
- Edited searchable metadata updates search.
- Archived/deleted projects are removed from search.
- One project can be reindexed.
- Entire index can be rebuilt from PostgreSQL.

## 58.5 Deployment

- System runs through Docker Compose.
- Nginx serves React production assets.
- Nginx proxies Go API.
- PostgreSQL persists through a volume.
- Meilisearch persists through a volume.
- Artifact files persist through managed storage.
- App works under a configured non-root base path.
- Refreshing nested React routes works.

## 58.6 Architecture Readiness

- UI strings use localization infrastructure.
- colors use centralized design tokens;
- project identity uses UUID;
- legacy code is separate reference data;
- Meilisearch is not canonical storage;
- report contents are not indexed;
- no Redis required;
- no message queue required;
- no microservices required.

---

# 59. Intentionally Deferred Decisions

The following are deliberately not frozen by this document:

- exact category vocabulary;
- exact platform vocabulary;
- exact domain vocabulary;
- exact topic vocabulary;
- exact technology vocabulary;
- exact CSV columns;
- exact XLSX template;
- exact canonical import JSON shape;
- exact Go router;
- exact Go SQL/ORM/query library;
- exact React data-fetching library;
- exact React form library;
- exact migration tool;
- exact URL slug scheme;
- exact artifact directory fan-out;
- exact upload-size limits;
- exact session duration;
- exact password policy;
- exact color hex values;
- exact logo placement;
- exact production Nginx configuration;
- backup format;
- backup destination;
- backup retention;
- backup encryption;
- Entra role mapping;
- organization-only access policy;
- analytics event schema;
- report full-text search;
- semantic search.

These belong to implementation planning or later stages.

---

# 60. Recommended MVP Technology Baseline

## Frontend
- React
- Vite
- React Router
- mature i18n library such as react-i18next
- centralized semantic design tokens

## Backend
- Go
- REST API
- modular monolith

## Database
- PostgreSQL

## Search
- Meilisearch

## Reverse Proxy and Static Serving
- Nginx

## Deployment
- Docker
- Docker Compose
- university VM
- images from GHCR, Docker Hub, or approved registry

## Artifact Storage
- local persistent filesystem/volume

## Authentication
MVP:
- local admin accounts;
- Argon2id;
- server-side session cookies.

Future:
- Microsoft Entra ID.

---

# 61. Indicative Repository Shape

```text
/
├── frontend/
├── backend/
├── deploy/
│   ├── docker-compose.yml
│   └── nginx.example.conf
├── docs/
├── scripts/
├── .env.example
└── README.md
```

Exact organization may change.

---

# 62. Design Principle Summary

1. Search first.
2. PostgreSQL is the source of truth.
3. Meilisearch is disposable derived state.
4. Project UUIDs are canonical identity.
5. Legacy project codes are reference values only.
6. People are entities, not embedded strings.
7. Taxonomy keys are stable while labels remain changeable.
8. Classifications may be multi-valued.
9. Artifacts are flexible child records.
10. PDFs use browser-native viewing.
11. PowerPoints are download-only.
12. Manual and bulk import share one validation pipeline.
13. AI extraction is outside this system.
14. Public reading is open in MVP.
15. Admin operations require local authentication.
16. Destructive actions require deliberate confirmation.
17. Normal deletion does not immediately imply physical destruction.
18. No recommendation engine in MVP.
19. No analytics in MVP.
20. No report-body search in MVP.
21. No Redis in MVP.
22. No message queue in MVP.
23. No microservices.
24. Deployment must support configurable base path.
25. Localization and dark mode are future-ready, not fully implemented.
26. Replaceable boundaries should remain small and practical.
27. Prefer maintainability over speculative abstraction.
28. Security and recoverability matter more than ornamental architecture.

---

# 63. Final MVP Product Statement

The MVP is a public, searchable, faceted institutional archive of senior projects with secure administrator maintenance tools.

It allows students and faculty to discover historical work by project metadata, people, academic context, and controlled classifications, then access the artifacts associated with those projects.

It deliberately excludes recommendation, analytics, AI extraction, semantic search, report-body search, custom document rendering, and complex infrastructure.

The baseline architecture uses React, Go, PostgreSQL, Meilisearch, Nginx, and Docker Compose on a university VM. PostgreSQL remains authoritative, Meilisearch remains derived, artifacts remain independently stored, and the application is structured so that future Entra authentication, institutional access controls, analytics, backup workflows, localization, dark mode, and richer search capabilities can be introduced without replacing the core design.

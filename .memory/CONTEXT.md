# AUSE Discovery

AUSE Discovery is the domain of preserving and discovering historical senior projects and associated academic records at Assumption University.

## Project orientation

AUSE Discovery is a public searchable institutional archive of historical senior projects. Public visitors search and browse Project metadata, people, academic context, controlled classifications, and available Artifacts. Authenticated administrators maintain canonical records and search state.

Canonical boundaries:

- PostgreSQL is canonical.
- Meilisearch is disposable derived state.
- Artifact and Project Logo bytes use the configured storage provider, local persistent storage or Backblaze B2, outside PostgreSQL. Provider selection is configuration, per the accepted pluggable-storage decision.
- Project is the search-result unit. Project ID is application-owned UUID identity.
- Reference Code is optional external data without assumed uniqueness or encoded semantics.
- Person records remain separate from Application Users.
- Student ID is public, searchable, and exactly seven digits.
- Delete is reversible soft deletion. Permanent destruction is outside MVP scope.

Outside MVP scope: recommendations, analytics, semantic search, report-body indexing, AI extraction, public accounts, taxonomy CMS, custom document viewers, Redis, message brokers, microservices, external-link Artifacts, automatic backup, and automated VM deployment.

## Projects

**Project**:
One historical senior project. A Project is the unit of discovery and owns academic context, participation, classifications, and Artifacts.
_Avoid_: Document, submission, search result record

**Project ID**:
The canonical application-owned identity of a Project.
_Avoid_: Reference code, legacy ID

**Reference Code**:
An optional external university reference associated with a Project. A Reference Code is not canonical identity and does not imply uniqueness or encoded meaning.
_Avoid_: Project ID

**Project Status**:
The current visibility lifecycle state of a Project: draft, published, or deleted.
_Avoid_: Archive status

**Delete**:
A reversible change that hides a Project from public access while preserving the record.
_Avoid_: Archive, hard delete, permanent deletion

**Restore**:
A reversal of Delete that returns a Project to the status held before deletion.
_Avoid_: Undelete, unarchive

## People

**Person**:
An academic person associated with one or more Projects. Institutional identifiers establish identity when present; otherwise, one unambiguous exact normalized-name match represents the same Person.
_Avoid_: User, account

**Student ID**:
The public seven-digit institutional identifier associated with a student Person.
_Avoid_: Student number, Person ID

**Project Participation**:
The relationship between a Person and a Project in the role of student, advisor, co-advisor, or committee member.
_Avoid_: Membership, embedded person

**Application User**:
An authenticated administrative identity permitted to maintain AUSE Discovery records.
_Avoid_: Person, administrator Person

## Classification and Academic Context

**Academic Catalog Value**:
A historically versioned Program, Major, or Course definition used by a Project.
_Avoid_: Tag, current curriculum value

**Taxonomy Value**:
A stable keyed classification within category, platform, domain, topic, or technology.
_Avoid_: Free-form tag

**Topic**:
An inactive Taxonomy Value dimension retained in storage, import, API, and search-filter infrastructure for possible future use. Topic is excluded from current interfaces and keyword matching because the collection lacks enough breadth for a useful controlled vocabulary. Expanding it would add overlapping classification and disproportionate maintenance.
_Avoid_: Public Topic filter, active Topic assignment

## Artifacts

**Artifact**:
A stored file associated with a Project, such as a report, slides, source archive, proposal, poster, dataset, or demo video. The internal domain term is Artifact. User-facing interfaces call it a Project file.
_Avoid_: Attachment URL, Project file column

**Artifact Store**:
The subsystem that persists and opens opaque Project-owned bytes for Artifact and Project Logo services. Sharing this infrastructure does not make a Project Logo a domain Artifact. A storage provider is a configured implementation, such as the local filesystem or Backblaze B2. A storage adapter is the code implementing that provider contract. `Backend` is reserved for the AUSE Backend API and must not describe an Artifact store or storage provider.
_Avoid_: Storage backend, B2 backend, filesystem backend

**Project Content Import**:
An operator-controlled import that maps existing Projects to Project Files, optional Project Logos, and optional Project Repository Links through one manifest after Project metadata exists. Each content item retains its own domain behavior after import.
_Avoid_: Metadata Import Batch, Logo Import, direct object-store copy

**Project Logo**:
An optional Project-owned representative image used for visual identification. Its bytes use the configured object store, but it is presentation metadata rather than an Artifact and never appears as a downloadable Project file.
_Avoid_: Logo Artifact, downloadable logo, Project attachment

**Project Repository Link**:
An external source-control repository identified as belonging to one Project, with last-checked availability metadata. It is not an Artifact, Project File, or third-party dependency reference.
_Avoid_: Repository Artifact, external-link Artifact, dependency link

## Import and Search

**Import Batch**:
The persistent preview and atomic commit boundary for one CSV or XLSX metadata upload.
_Avoid_: Temporary file, upload job

**Import Row**:
One proposed Project and associated metadata within an Import Batch.
_Avoid_: Database Project

**Search Document**:
The derived searchable representation of one published Project.
_Avoid_: Canonical Project record

**Search Filter Suggestion**:
An actionable completion shown from controlled academic, classification, or
Person facet values while a visitor composes a search query. Accepting a Search
Filter Suggestion adds the corresponding structured filter and removes the
recognized text from the query input. It suggests filters rather than running
result search on every keystroke.
_Avoid_: Tag, live result search, AI intent detection

**Search Synchronization**:
The projection of canonical Project data into the search index.
_Avoid_: Replication

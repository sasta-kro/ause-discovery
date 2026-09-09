# AUSE Discovery

AUSE Discovery is the domain of preserving and discovering historical senior projects and associated academic records at Assumption University.

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
An academic person associated with one or more Projects. A Person is separate from an authenticated administrative identity.
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

**Search Synchronization**:
The projection of canonical Project data into the search index.
_Avoid_: Replication

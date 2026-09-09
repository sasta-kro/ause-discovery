# Senior Project Corpus Analysis and Search Design

## Corpus processed

- 48 PDF reports
- 307 pages total
- 6.4 pages per report on average
- 11 pages maximum
- All 307 pages contained extractable text, so OCR was not required
- Text was extracted page-by-page and the PDFs were also rendered page-by-page for verification

## Main conclusion

Do not build one flat `tags[]` field and do not force every project into one exclusive `type` enum.

The reports routinely overlap categories. A project can simultaneously be a web application, an AI/ML project, a research experiment, and a retail project. The most useful design is a set of separate multi-valued facets.

In this corpus, curated labels produced these overlapping counts:

- software-system: 37 projects
- AI/ML: 14
- research/experiment: 9
- data/analytics: 5
- hardware/IoT: 3
- game: 3

Because the categories overlap, the total is intentionally greater than 48.

## Recommended metadata model

### 1. Identity and academic metadata

These should be structured fields, not tags.

- `project_id`
- `title`
- `title_aliases[]`
- `authors[]` as person entities
  - canonical name
  - display name
  - student ID
- `advisor` as a person entity
- `committee[]` as person entities
- `academic_year`
- `semester`
- `programs[]` such as CS and IT
- `course_codes[]`
- `school/faculty`
- `project_phase` such as Senior Project I, Senior Project II, proposal, final

### 2. Project mode

Multi-value controlled facet. Suggested initial vocabulary:

- `software-system`
- `AI/ML`
- `data/analytics`
- `research/experiment`
- `game`
- `hardware/IoT`
- `automation`

Do not make this a single enum.

### 3. Platform / deliverable form

Multi-value controlled facet:

- `web`
- `website`
- `mobile`
- `mobile web`
- `desktop`
- `web admin`
- `embedded/hardware`
- `wearable`
- `game engine`
- `online multiplayer`
- `data pipeline`
- `chatbot/web`

The vocabulary can be normalized further as the repository grows.

### 4. Domain

Use a small controlled top-level vocabulary with optional narrower children. Do not create a new top-level domain for every one-off project.

Suggested top-level groups:

- Education / Campus
- Business / Enterprise
- Retail / E-commerce
- Transportation / Mobility
- Health / Wellness
- Food / Hospitality
- Security / Surveillance
- Media / Gaming / Creative
- Travel / Culture
- Finance / Insurance
- Sports
- Energy / Utilities

Narrower domain tags can sit below these, for example `scholarships`, `library`, `jewelry`, `ride sharing`, `news`, or `fuel inventory`.

### 5. Technical method

This is one of the most useful tag layers for students looking for references.

Examples found in the corpus:

- computer vision
- object detection
- face detection
- video analytics
- NLP
- sentiment analysis
- topic modeling
- recommender system
- LLM
- RAG
- OCR
- image classification
- data augmentation
- PCG
- gamification
- geolocation
- Wi-Fi sensing
- biosensing
- workflow automation
- web scraping
- real-time rendering
- physics simulation

### 6. Technology used

Examples found in the reports include React, Next.js, Node.js, Laravel, PostgreSQL, MongoDB, Firebase, Kotlin, Jetpack Compose, Unity, Unreal Engine, OpenCV, TensorFlow, Keras, WordPress, GraphQL, n8n, Gemini, Sanity, and Raspberry Pi.

Keep two different concepts:

- `technologies_used[]`: confirmed implementation stack, safe to facet on
- `technologies_mentioned[]`: tools/models that merely occur in the report, literature review, or comparison

Do not convert every full-text technology mention into a displayed tag. One report may discuss YOLO or R-CNN as alternatives without actually using them.

### 7. Content fields

- `abstract`
- `reported_keywords[]`: keywords explicitly supplied by the authors
- `generated_keywords[]`: automatically extracted descriptive terms
- `full_text`

Generated keywords are useful for recall but should normally not become uncontrolled UI facets.

### 8. Artifact availability

For your actual repository, these fields are just as valuable as topic tags:

- `artifact_types[]`: report, slides, source code, dataset, poster, demo video, deployed app
- `has_report`
- `has_slides`
- `has_code`
- `has_dataset`
- `has_demo`
- repository URL
- deployed URL
- access level / visibility
- file sizes and MIME types

Users often want “show me projects with source code” more than another semantic topic tag.

## Important extraction problem found in the reports

Several reports contain stale templates or conflicting metadata.

Examples:

- 1719 is a MIDI project, but its approval page says `Text Classification for Education Publication`.
- 25102 is AU Canteen, but several page headers contain `Text Classification for Education Publication`.
- 26037 is `Game Realism through Graphics and Physics`, while the approval page says `Game IT Research` and stale page headers again contain the text-classification title.
- 26006 uses `RANGOON` throughout the report but the approval page says `YANGOON`.
- 2148 alternates between `Portfolio Builder` and `Resume Builder`.
- 26010 uses `AutoWise` as a header/team label while the report body and approval page describe a jewelry-shop management system.

Therefore, the ingestion pipeline should not treat one regex match from `Project title:` as ground truth.

Recommended ingestion workflow:

1. Extract candidates from cover, approval page, abstract, and page headers.
2. Assign a confidence score and preserve all aliases.
3. Prefer content-supported values over obvious template residue.
4. Send conflicts to an admin review queue.
5. Store the reviewed canonical value separately from the raw extracted candidates.

For people, use official university records as the best source when available. PDF extraction is useful for bootstrapping, but an official repository should not silently publish a misparsed author or advisor.

## Search indexing model

The top-level search result should be a project, not a PDF page and not an individual artifact.

Use one denormalized search document per project containing:

- title and title aliases
- author names and IDs
- advisor name
- academic metadata
- project modes
- platforms
- domains
- methods
- confirmed technologies
- abstract
- full report text
- artifact availability flags

Artifacts remain child records in the database/object store. You can later add page/chunk indexes for “search inside this project” or semantic retrieval, but ordinary search should return the project itself.

Suggested relative text ranking:

1. student ID exact match
2. title
3. title aliases
4. author names
5. advisor name
6. controlled tags
7. abstract
8. full report text

Full text should have a low weight. Otherwise a project can rank highly because a term appears once in its literature review or references.

## Search engine choice

For an official university application, keep PostgreSQL as the source of truth and treat the search engine as a rebuildable index.

A practical architecture is:

- PostgreSQL: normalized projects, people, academic metadata, artifacts, permissions
- Object storage: PDFs, slides, ZIPs, code archives, videos
- Background ingestion worker: text extraction, metadata candidate extraction, tag generation
- Typesense or Meilisearch: denormalized search index
- Web application: search UI + filters/facets

For this use case, Typesense or Meilisearch is a much more appropriate first dedicated search engine than Elasticsearch/OpenSearch unless your university already operates the latter. Both provide typo-tolerant full-text search and faceting without requiring you to build those primitives yourself.

PostgreSQL full-text search plus `pg_trgm` is also a valid minimal-infrastructure starting point. It is enough for a repository with hundreds or thousands of projects if you do not need sophisticated instant-search behavior. A dedicated engine becomes attractive when you want typo tolerance, facet counts, autocomplete, easier ranking tuning, and later semantic/hybrid search.

Do not add embeddings first. Start with correct metadata, aliases, lexical search, and facets. Semantic search is useful later for queries such as “projects about detecting how crowded a room is” when the exact report wording differs.

## The most relevant report to your planned website

Project 2117, `AuIdea`, is extremely close to the system you are describing. Its abstract says it was intended to let students view previous senior projects and advisor information, and it included a recommendation algorithm for suggesting projects. Inspect this project before finalizing your own information architecture because it is prior work on almost the same repository/discovery problem.

## Recommended first implementation

1. Create the relational schema and person entities.
2. Define the controlled vocabularies for `project_modes`, `platforms`, `domains`, and `methods`.
3. Ingest every project with raw extracted metadata plus a review status.
4. Manually review canonical title, people, and technologies-used for the initial corpus.
5. Index the reviewed records in Typesense or Meilisearch.
6. Search title, aliases, people, tags, abstract, and full text with different weights.
7. Expose facets for year, program, advisor, project mode, platform, domain, method, technology, and artifact availability.
8. Add synonyms and aliases after observing real failed queries.
9. Add semantic/hybrid search only after lexical relevance and metadata are working well.

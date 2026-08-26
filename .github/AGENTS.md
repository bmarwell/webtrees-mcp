# Contributor and agent guidance

## Architecture

Keep the application split into small ports and adapters:

- `internal/domain` contains genealogy concepts and business-facing data. It
  must not contain JSON tags, SQL details, or MCP types.
- `internal/genealogy` contains ports used by application code. Add interfaces
  here when a use case needs a replaceable data source.
- `internal/db` is the SQL/GEDCOM adapter. Keep row types, SQL, table prefixes,
  and GEDCOM parsing here, and map them to domain values.
- `internal/mcp` is the MCP adapter. Keep MCP schemas, JSON tags, response DTOs,
  tool descriptions, and domain-to-output mapping here.

Do not reuse database row structs as domain objects or MCP output DTOs. Do not
add an application service layer until a use case has actual orchestration or
business rules that need it.

## Tool descriptions

Every tool description must answer three questions in plain language:

1. `Use when ...` — identify the user question and required starting data.
2. `Do not use ...` or a limitation — prevent common misuse.
3. `Chain ...` — explain which returned stable IDs can be passed to another
   tool for verification or detail.

Descriptions must distinguish facts from research leads, explain date
semantics, and avoid promising data the adapter does not provide. Keep them
short enough for an agent to use during tool selection.

## MCP output contract

Successful results contain one concise human-readable `content` text item and
an object-shaped native `structuredContent` value matching the advertised
output schema. Do not put duplicate stringified JSON in `content`.

Collection payloads are wrapped in named objects such as `{ "people": [...] }`
or `{ "trees": [...] }`; do not return a top-level array as
`structuredContent`. Error results use diagnostic `content`, set the error
state through the MCP helper, and do not pretend to contain successful data.

Output DTOs should expose stable `tree_id`, `person_id`, and `family_id` values.
Preserve missing, uncertain, and partial facts explicitly. Never turn a
probable death, inferred relationship, or ranking based on incomplete dates
into an established fact.

When adding an endpoint or tool, publish its output contract in all three
places: define a transport-owned DTO with JSON tags in `internal/mcp/output.go`,
register the matching `WithOutputSchema[...]()` in `internal/mcp/tools.go`, and
document the shape with a representative example in `README.adoc`. Keep
collection results inside a named object, keep field names stable, and add a
test that checks both the generated schema and the structured result shape.
The human-readable `content` summary should explain the result without
duplicating its JSON.

## Research tool hints

When adding or changing a research tool, keep the human-readable result short:
state what was found, how it was ordered, and any important exclusions or
limitations. Put the complete machine-readable result in `structuredContent`
using a named object, not a JSON string or top-level array. Include stable IDs
and evidence fields so an agent can verify a lead with a follow-up tool. Keep
counts, bounds, confidence, missing facts, and date policies explicit; a
research lead is not an established fact.

## Genealogy terminology

- *Direct name hit*: the search query matches a primary, birth/maiden,
  married, or alternate name.
- *Indirect record hit*: the query matches other raw GEDCOM text, such as an
  occupation or note. These are excluded by default and require an explicit
  search option.
- *Family link*: an `FAMC` or `FAMS` connection between a person and a family
  record; it is not proof of a biological relationship by itself.
- *Evidence*: a parsed event, source reference, or explicit database link.
- *Research lead*: a useful hypothesis requiring source verification.
- *Recent*: a documented ordering policy, not an assertion that an event is
  historically the latest. State whether ordering uses year, exact date, or
  another policy.

## Chaining guidance

Use `list_tree_ids` when the tree is unknown. Use `search_persons` when starting
from a name, then pass selected IDs to `get_person`. Use `get_family` when a
family ID is known, then inspect its parent/child IDs with `get_person`.
Relationship and analysis tools must return the IDs and evidence needed for a
follow-up lookup. Agents should verify relationships through explicit family
links or a relationship-path tool rather than surname similarity.

Bound every search, traversal, and statistical query. Inspect limitations,
excluded counts, confidence, and date policy before presenting conclusions.

## Go style beyond gofmt

- Keep functions small and use early returns for validation and errors.
- Pass `context.Context` through I/O and future ports; do not hide request
  context in globals.
- Wrap errors with operation context using `%w`; do not discard database or
  close errors silently.
- Constrain every SQL query by tree ID and parameterize values. Validate table
  prefixes before interpolating identifiers.
- Prefer explicit conversion functions between DB, domain, and output types.
- Avoid clever reflection, global mutable state, and speculative abstractions.
- Keep user-facing text grammatical, factual, and consistent with optional
  fields. Add table-driven tests when phrases or output fields are conditional.
- Tests should be readable fixtures that state the behavior they protect;
  `sqlmock` is appropriate for adapter query behavior and small fakes are
  appropriate for port-level behavior.

Run `gofmt`, `go test ./...`, `go vet ./...`, and `git diff --check` before
committing.

## Commits

Keep each commit focused and use a concise imperative subject. For issue work,
use the repository convention `[#123] Add ...; fixes #123` (or `refs #123` when
the change does not fully resolve the issue). For work without an issue, use a
semantic-style subject such as `fix(mcp): reject invalid pagination`. Keep
documentation-only or agent-guidance changes in their own commit when they are
separate from a code change.

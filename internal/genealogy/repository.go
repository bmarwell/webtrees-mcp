package genealogy

import "webtrees-mcp/internal/domain"

// Repository is the application port used by MCP handlers. Adapters may use
// SQL, fixtures, or another data source without leaking their representations.
type Repository interface {
	GetPerson(treeID, personID string) (*domain.Person, error)
	SearchPersons(treeID, surname string, includeIndirect bool, limit, offset int) ([]domain.PersonSearchResult, error)
	GetFamily(treeID, familyID string) (*domain.Family, error)
	SearchEvents(treeID, eventType, fromDate, toDate, place string, limit, offset int) ([]domain.EventSearchResult, error)
	ListTrees(limit, offset int) ([]domain.Tree, error)
	ListRecentlyBorn(treeID string, limit, offset int) ([]domain.Person, error)
	ListRecentlyDeceased(treeID string, limit, offset int) ([]domain.Person, error)
}

const (
	DefaultPageSize = 10
	MaxPageSize     = 100
	MaxPageOffset   = 10000
)

func NormalizePage(limit, offset int) (int, int) {
	if limit < 1 {
		limit = DefaultPageSize
	}
	if limit > MaxPageSize {
		limit = MaxPageSize
	}
	if offset < 0 {
		offset = 0
	}
	if offset > MaxPageOffset {
		offset = MaxPageOffset
	}
	return limit, offset
}

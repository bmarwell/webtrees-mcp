package genealogy

import "webtrees-mcp/internal/domain"

// Repository is the application port used by MCP handlers. Adapters may use
// SQL, fixtures, or another data source without leaking their representations.
type Repository interface {
	GetPerson(treeID, personID string) (*domain.Person, error)
	SearchPersons(criteria PersonSearchCriteria) (PersonSearchResults, error)
	GetFamily(treeID, familyID string) (*domain.Family, error)
	SearchEvents(treeID, eventType, fromDate, toDate, place string, limit, offset int) ([]domain.EventSearchResult, error)
	ListRecentlyBorn(treeID string, limit, offset int) ([]domain.Person, error)
	ListRecentlyDeceased(treeID string, limit, offset int) ([]domain.Person, error)
}

// PersonSearchCriteria contains only indexed candidate filters. Nil year
// bounds mean that the corresponding bound is not applied.
type PersonSearchCriteria struct {
	TreeID          string
	Surname         string
	GivenName       string
	MatchMode       string
	Sex             string
	BirthYearMin    *int
	BirthYearMax    *int
	DeathYearMin    *int
	DeathYearMax    *int
	IncludeIndirect bool
	Limit           int
	Offset          int
}

// PersonSearchResults contains one page and the total number of matching
// indexed candidates.
type PersonSearchResults struct {
	People     []domain.PersonSearchResult
	TotalCount int
}

const (
	DefaultPageSize    = 10
	MaxPageSize        = 100
	MaxPageOffset      = 10000
	MaxFuzzyCandidates = 1000
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

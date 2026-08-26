package genealogy

import "webtrees-mcp/internal/domain"

// Repository is the application port used by MCP handlers. Adapters may use
// SQL, fixtures, or another data source without leaking their representations.
type Repository interface {
	GetPerson(treeID, personID string) (*domain.Person, error)
	SearchPersons(treeID, surname string, includeIndirect bool) ([]domain.PersonSearchResult, error)
	GetFamily(treeID, familyID string) (*domain.Family, error)
	ListTrees() ([]domain.Tree, error)
	ListRecentlyBorn(treeID string, limit int) ([]domain.Person, error)
	ListRecentlyDeceased(treeID string, limit int) ([]domain.Person, error)
}

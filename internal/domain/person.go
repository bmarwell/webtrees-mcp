package domain

// Name is a person's parsed name.
type Name struct {
	Given   string
	Surname string
	Type    string
}

// Relative is a link to another person without embedding that person's data.
type Relative struct {
	PersonID     string
	URL          string
	Relationship string
}

// FamilyLink connects an individual to an individual-as-child or
// individual-as-spouse family record.
type FamilyLink struct {
	FamilyID string
	Role     string
}

// Date preserves the source text and its explicit GEDCOM qualifier.
type Date struct {
	Raw       string
	Precision string
}

type Source struct {
	ID    string
	Title string
}

type Event struct {
	Type    string
	Date    *Date
	Place   string
	Value   string
	Notes   []string
	Sources []Source
}

// Person is the business representation of an individual.
type Person struct {
	ID          string
	Name        Name
	Names       []Name
	Sex         string
	BirthDate   string
	DeathDate   string
	Occupation  string
	Relatives   []Relative
	FamilyLinks []FamilyLink
	Events      []Event
	Notes       []string
	Sources     []Source
}

type SearchMatch struct {
	DirectHit bool
	Fields    []string
}

type PersonSearchResult struct {
	Person Person
	Match  SearchMatch
}

type Tree struct {
	ID    int
	Name  string
	Title string
}

type Family struct {
	ID       string
	Parents  []Relative
	Children []Relative
	Events   []Event
	Notes    []string
	Sources  []Source
}

// RelationshipPathStep records one explicitly evidenced family hop.
type RelationshipPathStep struct {
	FromPersonID string
	ToPersonID   string
	FamilyID     string
	Relationship string
}

// EventSearchResult is an event matched through the webtrees date index.
type EventSearchResult struct {
	PersonID string
	Type     string
	Date     string
	Place    string
}

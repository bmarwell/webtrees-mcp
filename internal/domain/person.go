package domain

// Name is a person's parsed name.
type Name struct {
	Given   string
	Surname string
}

// Relative is a link to another person without embedding that person's data.
type Relative struct {
	PersonID     string
	URL          string
	Relationship string
}

// Person is the business representation of an individual.
type Person struct {
	ID         string
	Name       Name
	Sex        string
	BirthDate  string
	DeathDate  string
	Occupation string
	Relatives  []Relative
}

type Tree struct {
	ID    string
	Name  string
	Title string
}

type Family struct {
	ID       string
	Parents  []Relative
	Children []Relative
}

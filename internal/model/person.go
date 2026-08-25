package model

// NameDTO is the parsed personal name from an individual record.
type NameDTO struct {
	Given   string `json:"given,omitempty"`
	Surname string `json:"surname,omitempty"`
}

// RelativeLink deliberately contains no embedded person data.
type RelativeLink struct {
	PersonID     string `json:"person_id"`
	URL          string `json:"url,omitempty"`
	Relationship string `json:"relationship,omitempty"`
}

// PersonDTO is the small, read-only representation exposed through MCP.
type PersonDTO struct {
	ID         string         `json:"id"`
	Name       NameDTO        `json:"name"`
	Sex        string         `json:"sex,omitempty"`
	BirthDate  string         `json:"birth_date,omitempty"`
	DeathDate  string         `json:"death_date,omitempty"`
	Occupation string         `json:"occupation,omitempty"`
	Relatives  []RelativeLink `json:"relatives,omitempty"`
}

type TreeDTO struct {
	ID    string `json:"tree_id"`
	Name  string `json:"name,omitempty"`
	Title string `json:"title,omitempty"`
}

type FamilyDTO struct {
	ID       string         `json:"family_id"`
	Parents  []RelativeLink `json:"parents,omitempty"`
	Children []RelativeLink `json:"children,omitempty"`
}

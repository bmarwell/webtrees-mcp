package mcp

import "webtrees-mcp/internal/domain"

// These are transport DTOs. They deliberately own the MCP/JSON contract
// instead of exposing domain or database representations to clients.
type personResultDTO struct {
	ID         string           `json:"id"`
	Name       nameOutput       `json:"name"`
	Sex        string           `json:"sex,omitempty"`
	BirthDate  string           `json:"birth_date,omitempty"`
	DeathDate  string           `json:"death_date,omitempty"`
	Occupation string           `json:"occupation,omitempty"`
	Relatives  []relativeOutput `json:"relatives,omitempty"`
}

type nameOutput struct {
	Given   string `json:"given,omitempty"`
	Surname string `json:"surname,omitempty"`
}

type relativeOutput struct {
	PersonID     string `json:"person_id"`
	URL          string `json:"url,omitempty"`
	Relationship string `json:"relationship,omitempty"`
}

type familyOutputDTO struct {
	ID       string           `json:"family_id"`
	Parents  []relativeOutput `json:"parents,omitempty"`
	Children []relativeOutput `json:"children,omitempty"`
}

type treeOutput struct {
	ID    string `json:"tree_id"`
	Name  string `json:"name,omitempty"`
	Title string `json:"title,omitempty"`
}

func personResult(person domain.Person) personResultDTO {
	return personResultDTO{
		ID:   person.ID,
		Name: nameOutput{Given: person.Name.Given, Surname: person.Name.Surname},
		Sex:  person.Sex, BirthDate: person.BirthDate, DeathDate: person.DeathDate,
		Occupation: person.Occupation, Relatives: relativeOutputs(person.Relatives),
	}
}

func personOutputs(people []domain.Person) []personResultDTO {
	outputs := make([]personResultDTO, 0, len(people))
	for _, person := range people {
		outputs = append(outputs, personResult(person))
	}
	return outputs
}

func relativeOutputs(relatives []domain.Relative) []relativeOutput {
	outputs := make([]relativeOutput, 0, len(relatives))
	for _, relative := range relatives {
		outputs = append(outputs, relativeOutput{
			PersonID: relative.PersonID, URL: relative.URL, Relationship: relative.Relationship,
		})
	}
	return outputs
}

func familyOutput(family domain.Family) familyOutputDTO {
	return familyOutputDTO{
		ID: family.ID, Parents: relativeOutputs(family.Parents), Children: relativeOutputs(family.Children),
	}
}

func treeOutputs(trees []domain.Tree) []treeOutput {
	outputs := make([]treeOutput, 0, len(trees))
	for _, tree := range trees {
		outputs = append(outputs, treeOutput{ID: tree.ID, Name: tree.Name, Title: tree.Title})
	}
	return outputs
}

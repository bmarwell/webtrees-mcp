package mcp

import "webtrees-mcp/internal/domain"

// These are transport DTOs. They deliberately own the MCP/JSON contract
// instead of exposing domain or database representations to clients.
type personResultDTO struct {
	ID             string             `json:"id"`
	Name           nameOutput         `json:"name"`
	AlternateNames []nameOutput       `json:"alternate_names,omitempty"`
	Sex            string             `json:"sex,omitempty"`
	BirthDate      string             `json:"birth_date,omitempty"`
	DeathDate      string             `json:"death_date,omitempty"`
	Occupation     string             `json:"occupation,omitempty"`
	Relatives      []relativeOutput   `json:"relatives,omitempty"`
	FamilyLinks    []familyLinkOutput `json:"family_links,omitempty"`
	Events         []eventOutput      `json:"events,omitempty"`
	Notes          []string           `json:"notes,omitempty"`
	Sources        []sourceOutput     `json:"sources,omitempty"`
	Match          *searchMatchOutput `json:"match,omitempty"`
}

type nameOutput struct {
	Given   string `json:"given,omitempty"`
	Surname string `json:"surname,omitempty"`
	Type    string `json:"type,omitempty"`
}

type relativeOutput struct {
	PersonID     string `json:"person_id"`
	URL          string `json:"url,omitempty"`
	Relationship string `json:"relationship,omitempty"`
}

type familyLinkOutput struct {
	FamilyID string `json:"family_id"`
	Role     string `json:"role"`
}

type dateOutput struct {
	Raw       string `json:"raw"`
	Precision string `json:"precision"`
}

type sourceOutput struct {
	ID    string `json:"id"`
	Title string `json:"title,omitempty"`
}

type eventOutput struct {
	Type    string         `json:"type"`
	Date    *dateOutput    `json:"date,omitempty"`
	Place   string         `json:"place,omitempty"`
	Value   string         `json:"value,omitempty"`
	Notes   []string       `json:"notes,omitempty"`
	Sources []sourceOutput `json:"sources,omitempty"`
}

type searchMatchOutput struct {
	DirectHit bool     `json:"direct_hit"`
	Fields    []string `json:"fields"`
}

type familyOutputDTO struct {
	ID       string           `json:"family_id"`
	Parents  []relativeOutput `json:"parents,omitempty"`
	Children []relativeOutput `json:"children,omitempty"`
	Events   []eventOutput    `json:"events,omitempty"`
	Notes    []string         `json:"notes,omitempty"`
	Sources  []sourceOutput   `json:"sources,omitempty"`
}

type treeOutput struct {
	ID    string `json:"tree_id"`
	Name  string `json:"name,omitempty"`
	Title string `json:"title,omitempty"`
}

type peopleResultDTO struct {
	People []personResultDTO `json:"people"`
}

type treesResultDTO struct {
	Trees []treeOutput `json:"trees"`
}

type relationshipPathStepOutput struct {
	FromPersonID string `json:"from_person_id"`
	ToPersonID   string `json:"to_person_id"`
	FamilyID     string `json:"family_id"`
	Relationship string `json:"relationship"`
}

type relationshipPathResultDTO struct {
	FromPersonID string                       `json:"from_person_id"`
	ToPersonID   string                       `json:"to_person_id"`
	Found        bool                         `json:"found"`
	Path         []relationshipPathStepOutput `json:"path"`
}

type eventSearchResultDTO struct {
	PersonID string `json:"person_id"`
	Type     string `json:"type"`
	Date     string `json:"date"`
	Place    string `json:"place,omitempty"`
}

type eventSearchResultsDTO struct {
	Events []eventSearchResultDTO `json:"events"`
}

func personResult(person domain.Person) personResultDTO {
	return personResultDTO{
		ID:             person.ID,
		Name:           nameOutput{Given: person.Name.Given, Surname: person.Name.Surname},
		AlternateNames: alternateNameOutputs(person.Names),
		Sex:            person.Sex, BirthDate: person.BirthDate, DeathDate: person.DeathDate,
		Occupation: person.Occupation, Relatives: relativeOutputs(person.Relatives),
		FamilyLinks: familyLinkOutputs(person.FamilyLinks), Events: eventOutputs(person.Events),
		Notes: person.Notes, Sources: sourceOutputs(person.Sources),
	}
}

func alternateNameOutputs(names []domain.Name) []nameOutput {
	if len(names) < 2 {
		return nil
	}
	outputs := make([]nameOutput, 0, len(names)-1)
	for _, name := range names[1:] {
		outputs = append(outputs, nameOutput{Given: name.Given, Surname: name.Surname, Type: name.Type})
	}
	return outputs
}

func searchPersonResult(result domain.PersonSearchResult) personResultDTO {
	output := personResult(result.Person)
	output.Match = &searchMatchOutput{DirectHit: result.Match.DirectHit, Fields: result.Match.Fields}
	return output
}

func searchPersonOutputs(results []domain.PersonSearchResult) []personResultDTO {
	outputs := make([]personResultDTO, 0, len(results))
	for _, result := range results {
		outputs = append(outputs, searchPersonResult(result))
	}
	return outputs
}

func personOutputs(people []domain.Person) []personResultDTO {
	outputs := make([]personResultDTO, 0, len(people))
	for _, person := range people {
		outputs = append(outputs, personResult(person))
	}
	return outputs
}

func peopleResult(people []domain.Person) peopleResultDTO {
	return peopleResultDTO{People: personOutputs(people)}
}

func peopleResultFromOutputs(people []personResultDTO) peopleResultDTO {
	return peopleResultDTO{People: people}
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

func familyLinkOutputs(links []domain.FamilyLink) []familyLinkOutput {
	outputs := make([]familyLinkOutput, 0, len(links))
	for _, link := range links {
		outputs = append(outputs, familyLinkOutput{FamilyID: link.FamilyID, Role: link.Role})
	}
	return outputs
}

func eventOutputs(events []domain.Event) []eventOutput {
	outputs := make([]eventOutput, 0, len(events))
	for _, event := range events {
		var date *dateOutput
		if event.Date != nil {
			date = &dateOutput{Raw: event.Date.Raw, Precision: event.Date.Precision}
		}
		outputs = append(outputs, eventOutput{
			Type: event.Type, Date: date, Place: event.Place, Value: event.Value,
			Notes: event.Notes, Sources: sourceOutputs(event.Sources),
		})
	}
	return outputs
}

func sourceOutputs(sources []domain.Source) []sourceOutput {
	outputs := make([]sourceOutput, 0, len(sources))
	for _, source := range sources {
		outputs = append(outputs, sourceOutput{ID: source.ID, Title: source.Title})
	}
	return outputs
}

func familyOutput(family domain.Family) familyOutputDTO {
	return familyOutputDTO{
		ID: family.ID, Parents: relativeOutputs(family.Parents), Children: relativeOutputs(family.Children),
		Events: eventOutputs(family.Events), Notes: family.Notes, Sources: sourceOutputs(family.Sources),
	}
}

func treeOutputs(trees []domain.Tree) []treeOutput {
	outputs := make([]treeOutput, 0, len(trees))
	for _, tree := range trees {
		outputs = append(outputs, treeOutput{ID: tree.ID, Name: tree.Name, Title: tree.Title})
	}
	return outputs
}

func treesResult(trees []domain.Tree) treesResultDTO {
	return treesResultDTO{Trees: treeOutputs(trees)}
}

func relationshipPathResult(fromID, toID string, found bool, path []domain.RelationshipPathStep) relationshipPathResultDTO {
	steps := make([]relationshipPathStepOutput, 0, len(path))
	for _, step := range path {
		steps = append(steps, relationshipPathStepOutput{FromPersonID: step.FromPersonID, ToPersonID: step.ToPersonID, FamilyID: step.FamilyID, Relationship: step.Relationship})
	}
	return relationshipPathResultDTO{FromPersonID: fromID, ToPersonID: toID, Found: found, Path: steps}
}

func eventSearchResults(events []domain.EventSearchResult) eventSearchResultsDTO {
	outputs := make([]eventSearchResultDTO, 0, len(events))
	for _, event := range events {
		outputs = append(outputs, eventSearchResultDTO{PersonID: event.PersonID, Type: event.Type, Date: event.Date, Place: event.Place})
	}
	return eventSearchResultsDTO{Events: outputs}
}

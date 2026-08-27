package mcp

import (
	"strings"

	"webtrees-mcp/internal/domain"
)

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
	AIContext      aiContextDTO       `json:"_ai_context"`
}

type aiContextDTO struct {
	ParentsFound []string `json:"parents_found,omitempty"`
	SpousesFound []string `json:"spouses_found,omitempty"`
	Hint         string   `json:"hint"`
	NextAction   string   `json:"next_action"`
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

type familyChildOutput struct {
	PersonID  string `json:"person_id"`
	Name      string `json:"name,omitempty"`
	BirthYear *int   `json:"birth_year,omitempty"`
	Sex       string `json:"sex,omitempty"`
}

type familyLinkOutput struct {
	FamilyID      string               `json:"family_id"`
	Role          string               `json:"role"`
	FamilyName    string               `json:"family_name,omitempty"`
	Parents       []familyPersonOutput `json:"parents,omitempty"`
	Spouse        *familyPersonOutput  `json:"spouse,omitempty"`
	ChildrenCount *int                 `json:"children_count,omitempty"`
}

type familyPersonOutput struct {
	PersonID string `json:"person_id"`
	Name     string `json:"name,omitempty"`
	Role     string `json:"role,omitempty"`
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
	ID        string              `json:"family_id"`
	Name      string              `json:"name,omitempty"`
	Parents   []relativeOutput    `json:"parents,omitempty"`
	Children  []familyChildOutput `json:"children,omitempty"`
	Events    []eventOutput       `json:"events,omitempty"`
	Notes     []string            `json:"notes,omitempty"`
	Sources   []sourceOutput      `json:"sources,omitempty"`
	AIContext aiContextDTO        `json:"_ai_context"`
}

// AIContext is added by handlers to guide the next bounded tool call.

type peopleResultDTO struct {
	People    []personResultDTO `json:"people"`
	AIContext aiContextDTO      `json:"_ai_context"`
}

type searchPeopleResultDTO struct {
	People     []personResultDTO `json:"people"`
	TotalCount int               `json:"total_count"`
	HasMore    bool              `json:"has_more"`
	Limit      int               `json:"limit"`
	Offset     int               `json:"offset"`
	AIContext  aiContextDTO      `json:"_ai_context"`
}

func searchPeopleResult(results []domain.PersonSearchResult, totalCount, limit, offset int) searchPeopleResultDTO {
	return searchPeopleResultDTO{
		People: searchPersonOutputs(results), TotalCount: totalCount,
		HasMore: offset+len(results) < totalCount, Limit: limit, Offset: offset,
	}
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
	AIContext    aiContextDTO                 `json:"_ai_context"`
}

type lineageNodeOutput struct {
	PersonID     string          `json:"person_id"`
	Depth        int             `json:"depth"`
	ViaFamilyID  string          `json:"via_family_id"`
	Relationship string          `json:"relationship"`
	Person       personResultDTO `json:"person"`
}

type lineageResultDTO struct {
	RootPersonID string              `json:"root_person_id"`
	Direction    string              `json:"direction"`
	Nodes        []lineageNodeOutput `json:"nodes"`
	Truncated    bool                `json:"truncated"`
	AIContext    aiContextDTO        `json:"_ai_context"`
}

type eventSearchResultDTO struct {
	PersonID string `json:"person_id"`
	Type     string `json:"type"`
	Date     string `json:"date"`
	Place    string `json:"place,omitempty"`
}

type eventSearchResultsDTO struct {
	Events    []eventSearchResultDTO `json:"events"`
	AIContext aiContextDTO           `json:"_ai_context"`
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

func enrichedPersonResult(person domain.Person, families map[string]domain.Family, people map[string]domain.Person) personResultDTO {
	output := personResult(person)
	output.FamilyLinks = enrichedFamilyLinkOutputs(person, families, people)
	return output
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

func enrichedFamilyLinkOutputs(person domain.Person, families map[string]domain.Family, people map[string]domain.Person) []familyLinkOutput {
	outputs := make([]familyLinkOutput, 0, len(person.FamilyLinks))
	for _, link := range person.FamilyLinks {
		output := familyLinkOutput{FamilyID: link.FamilyID, Role: link.Role}
		family, ok := families[link.FamilyID]
		if !ok {
			outputs = append(outputs, output)
			continue
		}
		output.FamilyName = familyName(family, people)
		if link.Role == "child" {
			for _, parent := range family.Parents {
				entry := familyPersonOutput{PersonID: parent.PersonID, Role: parentRole(people[parent.PersonID])}
				if related, exists := people[parent.PersonID]; exists {
					entry.Name = displayName(related)
				}
				output.Parents = append(output.Parents, entry)
			}
		} else if link.Role == "spouse" {
			count := len(family.Children)
			output.ChildrenCount = &count
			for _, parent := range family.Parents {
				if parent.PersonID == person.ID {
					continue
				}
				entry := familyPersonOutput{PersonID: parent.PersonID}
				if related, exists := people[parent.PersonID]; exists {
					entry.Name = displayName(related)
				}
				output.Spouse = &entry
				break
			}
		}
		outputs = append(outputs, output)
	}
	return outputs
}

func parentRole(person domain.Person) string {
	switch strings.ToUpper(person.Sex) {
	case "M":
		return "father"
	case "F":
		return "mother"
	default:
		return "parent"
	}
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

func familyOutput(family domain.Family, children map[string]domain.Person) familyOutputDTO {
	childOutputs := make([]familyChildOutput, 0, len(family.Children))
	for _, child := range family.Children {
		output := familyChildOutput{PersonID: child.PersonID}
		if person, ok := children[child.PersonID]; ok {
			output.Name = displayName(person)
			output.BirthYear = birthYear(person.BirthDate)
			output.Sex = person.Sex
		}
		childOutputs = append(childOutputs, output)
	}
	return familyOutputDTO{
		ID: family.ID, Name: familyName(family, children), Parents: relativeOutputs(family.Parents), Children: childOutputs,
		Events: eventOutputs(family.Events), Notes: family.Notes, Sources: sourceOutputs(family.Sources),
	}
}

func familyName(family domain.Family, people map[string]domain.Person) string {
	names := make([]string, 0, len(family.Parents))
	for _, parent := range family.Parents {
		if person, ok := people[parent.PersonID]; ok {
			if name := displayName(person); name != "" {
				names = append(names, name)
			}
		}
	}
	if len(names) == 0 {
		return ""
	}
	return "Family of " + strings.Join(names, " and ")
}

func relationshipPathResult(fromID, toID string, found bool, path []domain.RelationshipPathStep) relationshipPathResultDTO {
	steps := make([]relationshipPathStepOutput, 0, len(path))
	for _, step := range path {
		steps = append(steps, relationshipPathStepOutput{FromPersonID: step.FromPersonID, ToPersonID: step.ToPersonID, FamilyID: step.FamilyID, Relationship: step.Relationship})
	}
	return relationshipPathResultDTO{FromPersonID: fromID, ToPersonID: toID, Found: found, Path: steps}
}

func lineageResult(result domain.LineageResult) lineageResultDTO {
	nodes := make([]lineageNodeOutput, 0, len(result.Nodes))
	for _, node := range result.Nodes {
		nodes = append(nodes, lineageNodeOutput{
			PersonID: node.Person.ID, Depth: node.Depth, ViaFamilyID: node.ViaFamilyID,
			Relationship: node.Relationship, Person: personResult(node.Person),
		})
	}
	return lineageResultDTO{RootPersonID: result.RootPersonID, Direction: result.Direction, Nodes: nodes, Truncated: result.Truncated}
}

func eventSearchResults(events []domain.EventSearchResult) eventSearchResultsDTO {
	outputs := make([]eventSearchResultDTO, 0, len(events))
	for _, event := range events {
		outputs = append(outputs, eventSearchResultDTO{PersonID: event.PersonID, Type: event.Type, Date: event.Date, Place: event.Place})
	}
	return eventSearchResultsDTO{Events: outputs}
}

package genealogy

import (
	"testing"

	"webtrees-mcp/internal/domain"
)

type relationshipRepository struct {
	people   map[string]*domain.Person
	families map[string]*domain.Family
}

func (r relationshipRepository) GetPerson(_ string, id string) (*domain.Person, error) {
	return r.people[id], nil
}

func (r relationshipRepository) SearchPersons(PersonSearchCriteria) (PersonSearchResults, error) {
	return PersonSearchResults{}, nil
}

func (r relationshipRepository) GetFamily(_ string, id string) (*domain.Family, error) {
	return r.families[id], nil
}

func (r relationshipRepository) SearchEvents(string, string, string, string, string, int, int) ([]domain.EventSearchResult, error) {
	return nil, nil
}

func (r relationshipRepository) ListRecentlyBorn(string, int, int) ([]domain.Person, error) {
	return nil, nil
}

func (r relationshipRepository) ListRecentlyDeceased(string, int, int) ([]domain.Person, error) {
	return nil, nil
}

func TestFindRelationshipPathUsesExplicitFamilyLinks(t *testing.T) {
	repository := relationshipRepository{
		people: map[string]*domain.Person{
			"I1": {ID: "I1", FamilyLinks: []domain.FamilyLink{{FamilyID: "F1", Role: "child"}}},
			"I2": {ID: "I2", FamilyLinks: []domain.FamilyLink{{FamilyID: "F1", Role: "child"}, {FamilyID: "F2", Role: "child"}}},
			"I3": {ID: "I3", FamilyLinks: []domain.FamilyLink{{FamilyID: "F2", Role: "child"}}},
		},
		families: map[string]*domain.Family{
			"F1": {ID: "F1", Parents: []domain.Relative{{PersonID: "I2"}}, Children: []domain.Relative{{PersonID: "I1"}}},
			"F2": {ID: "F2", Parents: []domain.Relative{{PersonID: "I3"}}, Children: []domain.Relative{{PersonID: "I2"}}},
		},
	}

	path, found, err := FindRelationshipPath(repository, "tree", "I1", "I3")
	if err != nil || !found {
		t.Fatalf("FindRelationshipPath() = found %v, error %v; want a path", found, err)
	}
	if len(path) != 2 || path[0].ToPersonID != "I2" || path[0].FamilyID != "F1" || path[0].Relationship != "parent" || path[1].ToPersonID != "I3" || path[1].FamilyID != "F2" || path[1].Relationship != "parent" {
		t.Fatalf("unexpected relationship path: %+v", path)
	}
}

func TestFindRelationshipPathReportsDisconnectedPeople(t *testing.T) {
	repository := relationshipRepository{
		people:   map[string]*domain.Person{"I1": {ID: "I1"}, "I2": {ID: "I2"}},
		families: map[string]*domain.Family{},
	}
	path, found, err := FindRelationshipPath(repository, "tree", "I1", "I2")
	if err != nil || found || len(path) != 0 {
		t.Fatalf("FindRelationshipPath() = path %+v, found %v, error %v; want no path", path, found, err)
	}
}

func TestFindLineageTraversesDirectLinesWithDepthAndEvidence(t *testing.T) {
	repository := relationshipRepository{
		people: map[string]*domain.Person{
			"I1": {ID: "I1", FamilyLinks: []domain.FamilyLink{{FamilyID: "F1", Role: "child"}, {FamilyID: "F2", Role: "spouse"}}},
			"I2": {ID: "I2"}, "I3": {ID: "I3"}, "I4": {ID: "I4"},
		},
		families: map[string]*domain.Family{
			"F1": {ID: "F1", Parents: []domain.Relative{{PersonID: "I2"}, {PersonID: "I3"}}, Children: []domain.Relative{{PersonID: "I1"}}},
			"F2": {ID: "F2", Parents: []domain.Relative{{PersonID: "I1"}}, Children: []domain.Relative{{PersonID: "I4"}}},
		},
	}
	ancestors, err := FindAncestors(repository, "42", "I1", 2, 10)
	if err != nil || len(ancestors.Nodes) != 2 || ancestors.Nodes[0].Person.ID != "I2" || ancestors.Nodes[0].Depth != 1 || ancestors.Nodes[0].ViaFamilyID != "F1" {
		t.Fatalf("unexpected ancestors: %+v, error %v", ancestors, err)
	}
	descendants, err := FindDescendants(repository, "42", "I1", 1, 10)
	if err != nil || len(descendants.Nodes) != 1 || descendants.Nodes[0].Person.ID != "I4" || descendants.Nodes[0].Relationship != "descendant" {
		t.Fatalf("unexpected descendants: %+v, error %v", descendants, err)
	}
}

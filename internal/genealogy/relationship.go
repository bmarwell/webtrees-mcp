package genealogy

import (
	"fmt"
	"sort"

	"webtrees-mcp/internal/domain"
)

const (
	maxRelationshipPathLength = 12
	maxRelationshipPathNodes  = 2000
)

// FindRelationshipPath follows explicit family links and returns the shortest
// path found. The boolean is false when the individuals are disconnected.
func FindRelationshipPath(reader Repository, treeID, fromID, toID string) ([]domain.RelationshipPathStep, bool, error) {
	if fromID == toID {
		return []domain.RelationshipPathStep{}, true, nil
	}
	type state struct {
		personID string
		path     []domain.RelationshipPathStep
	}
	queue := []state{{personID: fromID}}
	visited := map[string]bool{fromID: true}
	people := make(map[string]*domain.Person)
	families := make(map[string]*domain.Family)

	loadPerson := func(id string) (*domain.Person, error) {
		if person, ok := people[id]; ok {
			return person, nil
		}
		person, err := reader.GetPerson(treeID, id)
		if err != nil {
			return nil, fmt.Errorf("load person %s: %w", id, err)
		}
		people[id] = person
		return person, nil
	}
	loadFamily := func(id string) (*domain.Family, error) {
		if family, ok := families[id]; ok {
			return family, nil
		}
		family, err := reader.GetFamily(treeID, id)
		if err != nil {
			return nil, fmt.Errorf("load family %s: %w", id, err)
		}
		families[id] = family
		return family, nil
	}

	for len(queue) > 0 && len(visited) <= maxRelationshipPathNodes {
		current := queue[0]
		queue = queue[1:]
		if len(current.path) >= maxRelationshipPathLength {
			continue
		}
		person, err := loadPerson(current.personID)
		if err != nil {
			return nil, false, err
		}
		familyLinks := append([]domain.FamilyLink(nil), person.FamilyLinks...)
		sort.Slice(familyLinks, func(i, j int) bool { return familyLinks[i].FamilyID < familyLinks[j].FamilyID })
		for _, link := range familyLinks {
			family, err := loadFamily(link.FamilyID)
			if err != nil {
				return nil, false, err
			}
			neighbors := familyNeighbors(current.personID, link.FamilyID, *family)
			for _, neighbor := range neighbors {
				if visited[neighbor.ToPersonID] {
					continue
				}
				visited[neighbor.ToPersonID] = true
				path := append(append([]domain.RelationshipPathStep(nil), current.path...), neighbor)
				if neighbor.ToPersonID == toID {
					return path, true, nil
				}
				queue = append(queue, state{personID: neighbor.ToPersonID, path: path})
				if len(visited) >= maxRelationshipPathNodes {
					break
				}
			}
		}
	}
	return []domain.RelationshipPathStep{}, false, nil
}

func familyNeighbors(personID, familyID string, family domain.Family) []domain.RelationshipPathStep {
	isParent := false
	isChild := false
	for _, parent := range family.Parents {
		isParent = isParent || parent.PersonID == personID
	}
	for _, child := range family.Children {
		isChild = isChild || child.PersonID == personID
	}
	neighbors := make(map[string]string)
	if isChild {
		for _, parent := range family.Parents {
			if parent.PersonID != personID {
				neighbors[parent.PersonID] = "parent"
			}
		}
		for _, child := range family.Children {
			if child.PersonID != personID {
				neighbors[child.PersonID] = "sibling"
			}
		}
	}
	if isParent {
		for _, parent := range family.Parents {
			if parent.PersonID != personID {
				neighbors[parent.PersonID] = "spouse"
			}
		}
		for _, child := range family.Children {
			neighbors[child.PersonID] = "child"
		}
	}
	ids := make([]string, 0, len(neighbors))
	for id := range neighbors {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	steps := make([]domain.RelationshipPathStep, 0, len(ids))
	for _, id := range ids {
		steps = append(steps, domain.RelationshipPathStep{FromPersonID: personID, ToPersonID: id, FamilyID: familyID, Relationship: neighbors[id]})
	}
	return steps
}

package genealogy

import (
	"fmt"
	"sort"

	"webtrees-mcp/internal/domain"
)

const (
	DefaultLineageDepth = 5
	MaxLineageDepth     = 20
	DefaultLineageLimit = 100
	MaxLineageLimit     = 500
)

// FindLineage follows only explicit parent/child family links. It returns a
// flat, depth-labelled tree so callers do not need to resolve family links
// themselves. Results are deterministic and bounded by depth and limit.
func FindLineage(reader Repository, treeID, rootID, direction string, maxDepth, limit int) (domain.LineageResult, error) {
	if maxDepth < 1 {
		maxDepth = DefaultLineageDepth
	}
	if maxDepth > MaxLineageDepth {
		maxDepth = MaxLineageDepth
	}
	if limit < 1 {
		limit = DefaultLineageLimit
	}
	if limit > MaxLineageLimit {
		limit = MaxLineageLimit
	}
	root, err := reader.GetPerson(treeID, rootID)
	if err != nil {
		return domain.LineageResult{}, fmt.Errorf("load root person %s: %w", rootID, err)
	}
	result := domain.LineageResult{RootPersonID: rootID, Direction: direction, Nodes: make([]domain.LineageNode, 0, limit)}
	queue := []struct {
		personID string
		depth    int
	}{{rootID, 0}}
	visited := map[string]bool{rootID: true}
	people := map[string]*domain.Person{rootID: root}
	families := make(map[string]*domain.Family)

	loadPerson := func(id string) (*domain.Person, error) {
		if person, ok := people[id]; ok {
			return person, nil
		}
		person, err := reader.GetPerson(treeID, id)
		if err != nil {
			return nil, fmt.Errorf("load lineage person %s: %w", id, err)
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
			return nil, fmt.Errorf("load lineage family %s: %w", id, err)
		}
		families[id] = family
		return family, nil
	}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if current.depth >= maxDepth {
			continue
		}
		person, err := loadPerson(current.personID)
		if err != nil {
			return domain.LineageResult{}, err
		}
		links := append([]domain.FamilyLink(nil), person.FamilyLinks...)
		sort.Slice(links, func(i, j int) bool { return links[i].FamilyID < links[j].FamilyID })
		for _, link := range links {
			if (direction == "ancestors" && link.Role != "child") || (direction == "descendants" && link.Role != "spouse") {
				continue
			}
			family, err := loadFamily(link.FamilyID)
			if err != nil {
				return domain.LineageResult{}, err
			}
			candidates := family.Children
			relationship := "descendant"
			if direction == "ancestors" {
				candidates = family.Parents
				relationship = "ancestor"
			}
			sort.Slice(candidates, func(i, j int) bool { return candidates[i].PersonID < candidates[j].PersonID })
			for _, candidate := range candidates {
				if visited[candidate.PersonID] {
					continue
				}
				if len(result.Nodes) >= limit {
					result.Truncated = true
					return result, nil
				}
				visited[candidate.PersonID] = true
				candidatePerson, err := loadPerson(candidate.PersonID)
				if err != nil {
					return domain.LineageResult{}, err
				}
				depth := current.depth + 1
				result.Nodes = append(result.Nodes, domain.LineageNode{Person: *candidatePerson, Depth: depth, ViaFamilyID: link.FamilyID, Relationship: relationship})
				queue = append(queue, struct {
					personID string
					depth    int
				}{candidate.PersonID, depth})
			}
		}
	}
	return result, nil
}

func FindAncestors(reader Repository, treeID, rootID string, maxDepth, limit int) (domain.LineageResult, error) {
	return FindLineage(reader, treeID, rootID, "ancestors", maxDepth, limit)
}

func FindDescendants(reader Repository, treeID, rootID string, maxDepth, limit int) (domain.LineageResult, error) {
	return FindLineage(reader, treeID, rootID, "descendants", maxDepth, limit)
}

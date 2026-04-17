package assets

import (
	"errors"
	"fmt"
)

// Tree defines a skill/spell progression tree. Players spend major points to
// unlock spine nodes and minor points to unlock off-spine nodes. Capstones are
// mutually exclusive and each costs two major points.
//
// Spine nodes must be unlocked in the order they appear. Off-spine nodes list
// their requirements (spine nodes or other nodes) in Prereqs. Capstones require
// the final spine node and are mutually exclusive within the tree.
type Tree struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Spine       []Node `json:"spine"`
	Nodes       []Node `json:"nodes"`
	Capstones   []Node `json:"capstones"`
}

// Validate checks that the tree has a name, description, and all nodes are valid with no duplicate IDs.
func (t *Tree) Validate() error {
	var errs []error

	if t.Name == "" {
		errs = append(errs, fmt.Errorf("name is required"))
	}
	if t.Description == "" {
		errs = append(errs, fmt.Errorf("description is required"))
	}

	// Build the full set of node IDs before validating individual nodes so
	// that prereqs can reference any node in the tree.
	allIds := make(map[string]bool, len(t.Spine)+len(t.Nodes)+len(t.Capstones))

	for i, n := range t.Spine {
		if n.Id == "" {
			continue
		}
		if allIds[n.Id] {
			errs = append(errs, fmt.Errorf("spine[%d]: duplicate id %q", i, n.Id))
		}
		allIds[n.Id] = true
	}
	for i, n := range t.Nodes {
		if n.Id == "" {
			continue
		}
		if allIds[n.Id] {
			errs = append(errs, fmt.Errorf("nodes[%d]: duplicate id %q", i, n.Id))
		}
		allIds[n.Id] = true
	}
	for i, n := range t.Capstones {
		if n.Id == "" {
			continue
		}
		if allIds[n.Id] {
			errs = append(errs, fmt.Errorf("capstones[%d]: duplicate id %q", i, n.Id))
		}
		allIds[n.Id] = true
	}

	for i, n := range t.Spine {
		if err := n.validate(allIds); err != nil {
			errs = append(errs, fmt.Errorf("spine[%d]: %w", i, err))
		}
	}
	for i, n := range t.Nodes {
		if err := n.validate(allIds); err != nil {
			errs = append(errs, fmt.Errorf("nodes[%d]: %w", i, err))
		}
	}
	for i, n := range t.Capstones {
		if err := n.validate(allIds); err != nil {
			errs = append(errs, fmt.Errorf("capstones[%d]: %w", i, err))
		}
	}

	return errors.Join(errs...)
}

// Node is a single unlockable in a tree. Its position in the tree (Spine,
// Nodes, or Capstones) determines the point cost and mutual-exclusion rules.
// When MaxRank is greater than one, the node may be purchased that many times,
// each purchase costing one minor point and granting the perks again.
type Node struct {
	Id          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Prereqs     *Prereq `json:"prereqs,omitempty"`
	MaxRank     int     `json:"max_rank,omitempty"`
	Perks       []Perk  `json:"perks"`
}

// Rank returns the effective maximum rank for the node.
// An unset MaxRank (zero) is treated as one.
func (n *Node) Rank() int {
	if n.MaxRank < 1 {
		return 1
	}
	return n.MaxRank
}

func (n *Node) validate(allIds map[string]bool) error {
	var errs []error

	if n.Id == "" {
		errs = append(errs, fmt.Errorf("id is required"))
	}
	if n.Name == "" {
		errs = append(errs, fmt.Errorf("name is required"))
	}
	if len(n.Perks) == 0 {
		errs = append(errs, fmt.Errorf("at least one perk is required"))
	}
	if n.MaxRank < 0 {
		errs = append(errs, fmt.Errorf("max_rank cannot be negative"))
	}
	if n.Prereqs != nil {
		if err := n.Prereqs.validate(allIds); err != nil {
			errs = append(errs, fmt.Errorf("prereqs: %w", err))
		}
	}
	for i, p := range n.Perks {
		if err := p.validate(); err != nil {
			errs = append(errs, fmt.Errorf("perks[%d]: %w", i, err))
		}
	}

	return errors.Join(errs...)
}

// Prereq describes the conditions that must be met before a node can be unlocked.
// Type controls how child terms are combined:
//
//   - "and" (default): all terms must be satisfied unless N is set
//   - "or": at least one term must be satisfied unless N is set
//   - "not": none of the terms may be satisfied, i.e. NOT(OR(terms))
//
// N sets the minimum number of terms that must be satisfied for "and" and "or".
// When N is zero, it defaults to:
//   - "and": all terms
//   - "or": 1 term
//
// N must be omitted/zero for "not".
// Terms are a list of conditions; each term is either a leaf node reference or a nested group.
type Prereq struct {
	Type  string `json:"type,omitempty"` // "and", "or", or "not"; defaults to "and"
	N     int    `json:"n,omitempty"`    // min satisfied terms; 0 = type default (and=all, or=1). Must be 0 for "not"
	Terms []Term `json:"terms,omitempty"`
}

// Term is a single prerequisite condition. Exactly one of Node or Group must be set.
type Term struct {
	Node  string  `json:"node,omitempty"`  // leaf node id
	Group *Prereq `json:"group,omitempty"` // nested prereq group
}

func (p *Prereq) validate(allIds map[string]bool) error {
	var errs []error

	typ := p.Type
	if typ == "" {
		typ = "and"
	}

	// If prereqs exists (non-nil), require at least one term.
	// Use `Prereqs: nil` / omit prereqs for "no prerequisites".
	if len(p.Terms) == 0 {
		errs = append(errs, fmt.Errorf("empty prereq group: omit prereqs entirely for no requirements"))
		return errors.Join(errs...)
	}

	switch typ {
	case "and", "or":
		// ok
	case "not":
		// N is not meaningful for NOT; NOT is always "none of the terms satisfied".
		if p.N != 0 {
			errs = append(errs, fmt.Errorf("n has no effect for type %q; must be 0/omitted", typ))
		}
	default:
		errs = append(errs, fmt.Errorf("type must be \"and\", \"or\", or \"not\", got %q", p.Type))
	}

	if p.N < 0 {
		errs = append(errs, fmt.Errorf("n cannot be negative"))
	}

	// For "and"/"or", N (if set) cannot exceed number of terms.
	if (typ == "and" || typ == "or") && p.N > len(p.Terms) {
		errs = append(errs, fmt.Errorf("n (%d) cannot exceed the number of terms (%d)", p.N, len(p.Terms)))
	}

	// Validate terms and detect duplicates for leaf node references.
	seenNodes := make(map[string]struct{}, len(p.Terms))
	for i, t := range p.Terms {
		hasNode := t.Node != ""
		hasGroup := t.Group != nil

		if hasNode == hasGroup {
			// Either both set or neither set.
			errs = append(errs, fmt.Errorf("terms[%d]: exactly one of node or group must be set", i))
			continue
		}

		if hasNode {
			if _, dup := seenNodes[t.Node]; dup {
				errs = append(errs, fmt.Errorf("terms[%d]: duplicate node id %q", i, t.Node))
			} else {
				seenNodes[t.Node] = struct{}{}
			}
			if !allIds[t.Node] {
				errs = append(errs, fmt.Errorf("terms[%d]: node %q not found", i, t.Node))
			}
			continue
		}

		// hasGroup
		if err := t.Group.validate(allIds); err != nil {
			errs = append(errs, fmt.Errorf("terms[%d].group: %w", i, err))
		}
	}

	// Additional NOT constraint: must have at least one term (already ensured) and
	// NOT is defined as NOT(OR(terms)), so it's meaningful only with terms present.
	// (Nothing else required here.)

	return errors.Join(errs...)
}

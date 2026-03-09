package commands

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/display"
	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/storage"
)

// TreesHandlerFactory creates handlers for listing and viewing skill trees.
type TreesHandlerFactory struct {
	trees storage.Storer[*assets.Tree]
	pub   game.Publisher
}

func NewTreesHandlerFactory(trees storage.Storer[*assets.Tree], pub game.Publisher) *TreesHandlerFactory {
	return &TreesHandlerFactory{trees: trees, pub: pub}
}

func (f *TreesHandlerFactory) Spec() *HandlerSpec {
	return &HandlerSpec{
		Config: []ConfigRequirement{
			{Name: "tree", Required: false},
		},
	}
}

func (f *TreesHandlerFactory) ValidateConfig(config map[string]string) error {
	return nil
}

func (f *TreesHandlerFactory) Create() (CommandFunc, error) {
	return func(ctx context.Context, in *CommandInput) error {
		charId := in.Char.Id()
		if name := in.Config["tree"]; name != "" {
			return f.showTree(name, charId)
		}
		return f.listTrees(charId)
	}, nil
}

func (f *TreesHandlerFactory) listTrees(charId string) error {
	all := f.trees.GetAll()

	ids := make([]string, 0, len(all))
	for id := range all {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	lines := []string{"Skill Trees", ""}
	for _, id := range ids {
		t := all[id]
		lines = append(lines, fmt.Sprintf("  %-16s - %s", t.Name, firstSentence(t.Description)))
	}

	return f.publish(charId, strings.Join(lines, "\n"))
}

func (f *TreesHandlerFactory) showTree(name string, charId string) error {
	tree := f.findTree(name)
	if tree == nil {
		return NewUserError(fmt.Sprintf("No skill tree named %q.", name))
	}
	return f.publish(charId, renderTree(tree))
}

// findTree looks up a tree by ID first, then by name case-insensitively.
func (f *TreesHandlerFactory) findTree(name string) *assets.Tree {
	if t := f.trees.Get(strings.ToLower(name)); t != nil {
		return t
	}
	lower := strings.ToLower(name)
	for _, t := range f.trees.GetAll() {
		if strings.ToLower(t.Name) == lower {
			return t
		}
	}
	return nil
}

func (f *TreesHandlerFactory) publish(charId, text string) error {
	if f.pub != nil {
		return f.pub.Publish(game.SinglePlayer(charId), nil, []byte(text))
	}
	return nil
}

// renderTree produces a full text display of a skill tree.
func renderTree(t *assets.Tree) string {
	var lines []string

	lines = append(lines, t.Name)
	lines = append(lines, display.Wrap(t.Description))

	// Build lookup maps for spine tier resolution and name lookups.
	spineIdxById := make(map[string]int, len(t.Spine))
	for i, n := range t.Spine {
		spineIdxById[n.Id] = i
	}

	nameById := make(map[string]string, len(t.Spine)+len(t.Nodes)+len(t.Capstones))
	for _, n := range t.Spine {
		nameById[n.Id] = n.Name
	}
	for _, n := range t.Nodes {
		nameById[n.Id] = n.Name
	}
	for _, n := range t.Capstones {
		nameById[n.Id] = n.Name
	}

	// Spine section.
	lines = append(lines, "", "SPINE  [1 major point each]")
	for _, n := range t.Spine {
		lines = append(lines, fmt.Sprintf("  %-20s - %s", n.Name, firstSentence(n.Description)))
	}

	// Group regular nodes by their highest spine prereq tier.
	tierNodes := make(map[int][]*assets.Node)
	for i := range t.Nodes {
		n := &t.Nodes[i]
		tier := maxSpineTier(n.Prereqs, spineIdxById)
		tierNodes[tier] = append(tierNodes[tier], n)
	}

	tiers := make([]int, 0, len(tierNodes))
	for tier := range tierNodes {
		tiers = append(tiers, tier)
	}
	sort.Ints(tiers)

	for _, tier := range tiers {
		nodes := tierNodes[tier]
		sort.Slice(nodes, func(i, j int) bool { return nodes[i].Name < nodes[j].Name })

		if tier < 0 {
			lines = append(lines, "", "GENERAL  [1 minor point each]")
		} else {
			lines = append(lines, "", fmt.Sprintf("TIER %s  (requires %s)  [1 minor point each]",
				toRoman(tier+1), t.Spine[tier].Name))
		}

		for _, n := range nodes {
			label := n.Name
			if n.Rank() > 1 {
				label = fmt.Sprintf("%s x%d", label, n.Rank())
			}
			if req := nonSpinePrereqNames(n.Prereqs, spineIdxById, nameById); req != "" {
				label = fmt.Sprintf("%s  (req: %s)", label, req)
			}
			lines = append(lines, fmt.Sprintf("  %-40s - %s", label, firstSentence(n.Description)))
		}
	}

	// Capstones section.
	if len(t.Capstones) > 0 {
		lines = append(lines, "", "CAPSTONES  [2 major points, choose one]")
		for _, n := range t.Capstones {
			lines = append(lines, fmt.Sprintf("  %-20s - %s", n.Name, firstSentence(n.Description)))
		}
	}

	return strings.Join(lines, "\n")
}

// maxSpineTier returns the index of the highest spine node referenced anywhere
// in the prereq tree, or -1 if no spine nodes are referenced.
func maxSpineTier(p *assets.Prereq, spineIdxById map[string]int) int {
	if p == nil {
		return -1
	}
	max := -1
	for _, term := range p.Terms {
		if term.Node != "" {
			if idx, ok := spineIdxById[term.Node]; ok && idx > max {
				max = idx
			}
		} else if term.Group != nil {
			if v := maxSpineTier(term.Group, spineIdxById); v > max {
				max = v
			}
		}
	}
	return max
}

// nonSpinePrereqNames returns the names of all non-spine nodes referenced in a prereq.
func nonSpinePrereqNames(p *assets.Prereq, spineIdxById map[string]int, nameById map[string]string) string {
	if p == nil {
		return ""
	}
	var names []string
	for _, id := range collectPrereqIds(p) {
		if _, isSpine := spineIdxById[id]; isSpine {
			continue
		}
		if name, ok := nameById[id]; ok {
			names = append(names, name)
		}
	}
	return strings.Join(names, ", ")
}

// collectPrereqIds returns all node IDs referenced in a prereq, recursively.
func collectPrereqIds(p *assets.Prereq) []string {
	var ids []string
	for _, term := range p.Terms {
		if term.Node != "" {
			ids = append(ids, term.Node)
		} else if term.Group != nil {
			ids = append(ids, collectPrereqIds(term.Group)...)
		}
	}
	return ids
}

// firstSentence returns text up to and including the first period, or the full
// text if no period is found.
func firstSentence(s string) string {
	if i := strings.Index(s, "."); i >= 0 {
		return s[:i+1]
	}
	return s
}

// toRoman converts small integers (1–8) to Roman numerals for tier labels.
func toRoman(n int) string {
	numerals := []string{"I", "II", "III", "IV", "V", "VI", "VII", "VIII"}
	if n >= 1 && n <= len(numerals) {
		return numerals[n-1]
	}
	return fmt.Sprintf("%d", n)
}

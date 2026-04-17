package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	mermaidascii "github.com/AlexanderGrooff/mermaid-ascii/cmd"
	"github.com/AlexanderGrooff/mermaid-ascii/pkg/diagram"
	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/storage"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: treetool <tree-id>\n")
		os.Exit(1)
	}

	tree, err := loadTree(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	markup := toMermaid(tree)

	if len(os.Args) > 2 && os.Args[2] == "--mermaid" {
		fmt.Print(markup)
		return
	}

	config := diagram.DefaultConfig()
	config.GraphDirection = "TD"
	config.PaddingBetweenX = 2
	config.PaddingBetweenY = 1

	result, err := mermaidascii.RenderDiagram(markup, config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "render error: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(result)
}

func loadTree(id string) (*assets.Tree, error) {
	data, err := os.ReadFile(filepath.Join("assets", "trees", id+".json"))
	if err != nil {
		return nil, fmt.Errorf("reading tree file: %w", err)
	}

	var asset storage.Asset[*assets.Tree]
	if err := json.Unmarshal(data, &asset); err != nil {
		return nil, fmt.Errorf("parsing tree file: %w", err)
	}

	return asset.Spec, nil
}

// toMermaid converts a skill tree into a Mermaid graph TD definition.
// mermaid-ascii uses node names directly (no [label] support), so we use
// display names as identifiers and build an ID-to-name map for edges.
func toMermaid(tree *assets.Tree) string {
	var b strings.Builder
	b.WriteString("graph TD\n")

	nameOf := buildNameMap(tree)
	spineIDs := buildSpineSet(tree)

	// Spine chain.
	for i := 0; i < len(tree.Spine)-1; i++ {
		fmt.Fprintf(&b, "    %s --> %s\n",
			nameOf[tree.Spine[i].Id], nameOf[tree.Spine[i+1].Id])
	}

	// Prereq edges for all non-spine nodes. Skip NOT groups (mutual-exclusion
	// constraints, not true dependencies) and edges pointing to spine nodes
	// (the spine chain already captures that progression).
	allNodes := append(tree.Nodes, tree.Capstones...)
	for _, n := range allNodes {
		if n.Prereqs == nil {
			continue
		}
		for _, pid := range positivePrereqIDs(n.Prereqs) {
			if spineIDs[pid] {
				continue
			}
			if _, ok := nameOf[pid]; ok {
				fmt.Fprintf(&b, "    %s --> %s\n", nameOf[pid], nameOf[n.Id])
			}
		}
	}

	return b.String()
}

func buildSpineSet(tree *assets.Tree) map[string]bool {
	m := make(map[string]bool, len(tree.Spine))
	for _, n := range tree.Spine {
		m[n.Id] = true
	}
	return m
}

// buildNameMap returns a map from node ID to display name for use in mermaid output.
func buildNameMap(tree *assets.Tree) map[string]string {
	m := make(map[string]string)
	for _, n := range tree.Spine {
		m[n.Id] = n.Name
	}
	for _, n := range tree.Nodes {
		name := n.Name
		if n.Rank() > 1 {
			name += fmt.Sprintf(" x%d", n.Rank())
		}
		m[n.Id] = name
	}
	for _, n := range tree.Capstones {
		m[n.Id] = n.Name
	}
	return m
}

// positivePrereqIDs extracts leaf node IDs from a prereq tree, skipping NOT
// groups (which represent mutual-exclusion constraints, not true dependencies).
func positivePrereqIDs(p *assets.Prereq) []string {
	typ := p.Type
	if typ == "" {
		typ = assets.PrereqAnd
	}
	if typ == assets.PrereqNot {
		return nil
	}

	var ids []string
	for _, t := range p.Terms {
		if t.Node != "" {
			ids = append(ids, t.Node)
		} else if t.Group != nil {
			ids = append(ids, positivePrereqIDs(t.Group)...)
		}
	}
	return ids
}


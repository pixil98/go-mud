package main

import (
	"fmt"
	"testing"

	mermaidascii "github.com/AlexanderGrooff/mermaid-ascii/cmd"
	"github.com/AlexanderGrooff/mermaid-ascii/pkg/diagram"
)

func TestSmallGraph(t *testing.T) {
	input := `graph TD
    A[Root] --> B[Left]
    A --> C[Right]
    B --> D[Deep Left]`
	config := diagram.DefaultConfig()
	config.GraphDirection = "TD"
	result, err := mermaidascii.RenderDiagram(input, config)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Print(result)
}

// self_healing_agent demonstrates using GEP (Gene Evolution Protocol) to capture and reuse fixes.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/linkerlin/agentscope.go/evolver"
)

func main() {
	ctx := context.Background()

	// MockEvolver runs locally without external dependencies.
	// In production, connect to the Evolver MCP backend via Gateway.
	client := evolver.NewMockEvolver()
	flow := evolver.NewGEPFlow(client)

	// Simulate a recurring incident.
	context := "recurring gateway timeout on large payload uploads"

	runCfg := evolver.RunConfig{
		Context:  context,
		Strategy: "repair-only",
	}

	runRes, solRes, err := flow.RunAndSolidify(ctx, runCfg, false)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("=== GEP Self-Healing Result ===")
	fmt.Printf("Selected gene: %s (category=%s)\n", runRes.SelectedGene.ID, runRes.SelectedGene.Category)
	fmt.Printf("Strategy: %s\n", runRes.SelectedGene.Strategy)
	fmt.Printf("Solidified capsule: %s\n", solRes.CapsuleID)

	// Remember the capsule for future recall.
	if err := flow.Client.Remember(ctx, evolver.RememberRequest{
		Text:     fmt.Sprintf("Fix for %s: use chunked upload with retry", context),
		Type:     "capsule",
		Category: evolver.CategoryRepair,
	}); err != nil {
		log.Fatal(err)
	}

	// Later, recall relevant fixes.
	hits, err := flow.Client.Recall(ctx, evolver.RecallRequest{
		Query:    "gateway timeout large payload",
		Category: "capsule",
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("\n=== Recall Hits ===")
	for _, h := range hits {
		fmt.Println(h.Text)
	}
}

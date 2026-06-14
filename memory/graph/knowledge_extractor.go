package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

// Triple represents a knowledge triple: (subject, relation, object).
type Triple struct {
	Subject  string `json:"subject"`
	Relation string `json:"relation"`
	Object   string `json:"object"`
}

// KnowledgeExtractor uses an LLM to extract entities and relations from text.
type KnowledgeExtractor struct {
	model model.ChatModel
}

// NewKnowledgeExtractor creates a KnowledgeExtractor backed by the given model.
func NewKnowledgeExtractor(m model.ChatModel) *KnowledgeExtractor {
	return &KnowledgeExtractor{model: m}
}

const knowledgeExtractionPrompt = `Extract knowledge entities and their relationships from the following text.
Return a JSON object with an "entities" array and a "triples" array.

Each entity: {"name": "...", "type": "concept|person|event|memory|source", "description": "..."}
Each triple: {"subject": "EntityA", "relation": "relates_to|derived_from|part_of|causes|contradicts|supports|mentions", "object": "EntityB"}

Rules:
- Use concise entity names (1-3 words).
- Only extract explicit or strongly implied relationships.
- Do not invent information not present in the text.
- Return at most 15 entities and 20 triples.

Text:
%s`

type extractionResponse struct {
	Entities []struct {
		Name        string `json:"name"`
		Type        string `json:"type"`
		Description string `json:"description"`
	} `json:"entities"`
	Triples []Triple `json:"triples"`
}

// Extract parses text and returns knowledge triples and entity descriptions.
func (e *KnowledgeExtractor) Extract(ctx context.Context, text string) ([]Triple, []EntityInfo, error) {
	if e.model == nil {
		return nil, nil, fmt.Errorf("graph: no model configured")
	}
	if strings.TrimSpace(text) == "" {
		return nil, nil, nil
	}

	msgs := []*message.Msg{
		message.NewMsg().Role(message.RoleSystem).TextContent(
			"You are a knowledge extraction assistant. Output valid JSON only, no markdown.").Build(),
		message.NewMsg().Role(message.RoleUser).TextContent(
			fmt.Sprintf(knowledgeExtractionPrompt, text)).Build(),
	}

	resp, err := e.model.Chat(ctx, msgs, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("graph: extraction LLM call: %w", err)
	}

	content := resp.GetTextContent()
	content = stripMarkdownFence(content)

	var result extractionResponse
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, nil, fmt.Errorf("graph: parse extraction result: %w", err)
	}

	entities := make([]EntityInfo, 0, len(result.Entities))
	for _, e := range result.Entities {
		entities = append(entities, EntityInfo{
			Name:        e.Name,
			Type:        NodeType(e.Type),
			Description: e.Description,
		})
	}

	return result.Triples, entities, nil
}

// EntityInfo describes an extracted entity.
type EntityInfo struct {
	Name        string
	Type        NodeType
	Description string
}

// ExtractAndAdd extracts triples from text and adds them to a graph.
// Entities become Nodes and triples become Edges. Existing nodes are not duplicated.
// Returns the list of created/updated nodes.
func (e *KnowledgeExtractor) ExtractAndAdd(ctx context.Context, text string, g *Graph) ([]*Node, error) {
	triples, entities, err := e.Extract(ctx, text)
	if err != nil {
		return nil, err
	}
	return addTriplesToGraph(g, triples, entities), nil
}

func addTriplesToGraph(g *Graph, triples []Triple, entities []EntityInfo) []*Node {
	entityMap := make(map[string]EntityInfo)
	for _, e := range entities {
		entityMap[strings.ToLower(e.Name)] = e
	}

	var created []*Node

	for _, t := range triples {
		subjectID := normalizeID(t.Subject)
		objectID := normalizeID(t.Object)

		if g.GetNode(subjectID) == nil {
			node := &Node{
				ID:      subjectID,
				Title:   t.Subject,
				Content: getDescription(entityMap, t.Subject),
				Type:    getType(entityMap, t.Subject),
			}
			_ = g.AddNode(node)
			created = append(created, node)
		}
		if g.GetNode(objectID) == nil {
			node := &Node{
				ID:      objectID,
				Title:   t.Object,
				Content: getDescription(entityMap, t.Object),
				Type:    getType(entityMap, t.Object),
			}
			_ = g.AddNode(node)
			created = append(created, node)
		}

		_ = g.AddEdge(&Edge{
			Source:        subjectID,
			Target:        objectID,
			Relation:      Relation(t.Relation),
			Weight:        1.0,
			Bidirectional: t.Relation == string(RelRelatedTo) || t.Relation == string(RelSupports),
		})
	}

	return created
}

func normalizeID(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "-", "_")
	return s
}

func getDescription(entities map[string]EntityInfo, name string) string {
	if e, ok := entities[strings.ToLower(name)]; ok && e.Description != "" {
		return e.Description
	}
	return ""
}

func getType(entities map[string]EntityInfo, name string) NodeType {
	if e, ok := entities[strings.ToLower(name)]; ok && e.Type != "" {
		return e.Type
	}
	return NodeTypeConcept
}

func stripMarkdownFence(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```json") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimSuffix(s, "```")
	} else if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimSuffix(s, "```")
	}
	return strings.TrimSpace(s)
}

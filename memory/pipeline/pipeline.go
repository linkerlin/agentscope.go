package pipeline

import (
	"context"
	"fmt"
	"sync"
)

// Step 流水线中的一个步骤
type Step interface {
	Name() string
	Execute(ctx context.Context, fc *FlowContext) error
}

// StepMode 步骤组合模式
type StepMode int

const (
	Sequential StepMode = iota
	Parallel
	Branch
)

// StepNode 步骤树节点
type StepNode struct {
	Step     Step
	Children []*StepNode
	Mode     StepMode
}

// Pipeline 可执行流水线
type Pipeline struct {
	Root *StepNode
	Name string
}

// NewPipeline 创建流水线
func NewPipeline(name string, root *StepNode) *Pipeline {
	return &Pipeline{Name: name, Root: root}
}

// Execute 执行完整流水线
func (p *Pipeline) Execute(ctx context.Context, fc *FlowContext) error {
	if p.Root == nil {
		return fmt.Errorf("pipeline %q: nil root", p.Name)
	}
	return p.executeNode(ctx, p.Root, fc)
}

func (p *Pipeline) executeNode(ctx context.Context, node *StepNode, fc *FlowContext) error {
	if node == nil {
		return nil
	}

	switch node.Mode {
	case Sequential:
		if node.Step != nil {
			if err := node.Step.Execute(ctx, fc); err != nil {
				return fmt.Errorf("step %q: %w", node.Step.Name(), err)
			}
		}
		for _, child := range node.Children {
			if err := p.executeNode(ctx, child, fc); err != nil {
				return err
			}
		}

	case Parallel:
		if node.Step != nil {
			if err := node.Step.Execute(ctx, fc); err != nil {
				return fmt.Errorf("step %q: %w", node.Step.Name(), err)
			}
		}
		var wg sync.WaitGroup
		errs := make([]error, len(node.Children))
		for i, child := range node.Children {
			wg.Add(1)
			go func(idx int, c *StepNode) {
				defer wg.Done()
				fcCopy := *fc
				errs[idx] = p.executeNode(ctx, c, &fcCopy)
			}(i, child)
		}
		wg.Wait()
		for _, err := range errs {
			if err != nil {
				return err
			}
		}

	case Branch:
		if node.Step != nil {
			if err := node.Step.Execute(ctx, fc); err != nil {
				return fmt.Errorf("step %q: %w", node.Step.Name(), err)
			}
		}
		if len(node.Children) > 0 {
			return p.executeNode(ctx, node.Children[0], fc)
		}
	}

	return nil
}

// 构建器函数

// Seq 串联步骤：按顺序执行所有子步骤
func Seq(steps ...Step) *StepNode {
	children := make([]*StepNode, len(steps))
	for i, s := range steps {
		children[i] = &StepNode{Step: s, Mode: Sequential}
	}
	return &StepNode{
		Mode:     Sequential,
		Children: children,
	}
}

// Par 并行步骤：并发执行所有子步骤
func Par(steps ...Step) *StepNode {
	children := make([]*StepNode, len(steps))
	for i, s := range steps {
		children[i] = &StepNode{Step: s, Mode: Sequential}
	}
	return &StepNode{
		Mode:     Parallel,
		Children: children,
	}
}

// BranchFirst 分支步骤：执行指定步骤后，若存在子节点则执行第一个子节点
func BranchFirst(step Step) *StepNode {
	return &StepNode{
		Step: step,
		Mode: Branch,
	}
}

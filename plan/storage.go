package plan

// Storage defines the persistent storage interface for plans.
type Storage interface {
	AddPlan(p *Plan) error
	GetPlan(planID string) (*Plan, error)
	ListPlans() ([]*Plan, error)
	DeletePlan(planID string) error
}

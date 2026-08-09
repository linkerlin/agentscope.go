package controlplane

// cloneGoal returns a deep copy of g so callers cannot mutate the store's
// internal slice fields (Scope, Authority) through a returned pointer (#4).
func cloneGoal(g *Goal) *Goal {
	if g == nil {
		return nil
	}
	cp := *g
	if g.Scope != nil {
		cp.Scope = append([]string(nil), g.Scope...)
	}
	if g.Authority != nil {
		cp.Authority = append([]AuthoritySrc(nil), g.Authority...)
	}
	return &cp
}

// cloneTodo returns a deep copy of t (Evidence slice isolated; EvidenceIDs is
// derived, no separate copy needed — #5 round-5).
func cloneTodo(t *Todo) *Todo {
	if t == nil {
		return nil
	}
	cp := *t
	if t.Evidence != nil {
		cp.Evidence = append([]Evidence(nil), t.Evidence...)
	}
	return &cp
}

// cloneGate returns a deep copy of g (Resolvers slice + Fallback/Outcome
// pointers isolated), so a caller mutating the returned gate cannot reach the
// store's internal state.
func cloneGate(g UserGate) UserGate {
	if g.Resolvers != nil {
		g.Resolvers = append([]string(nil), g.Resolvers...)
	}
	if g.Fallback != nil {
		fb := *g.Fallback
		g.Fallback = &fb
	}
	if g.Outcome != nil {
		oc := *g.Outcome
		g.Outcome = &oc
	}
	return g
}

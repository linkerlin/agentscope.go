package controlplane

import (
	"errors"
	"hash/fnv"
	"sort"
	"strings"
)

// AssignPeer deterministically selects exactly one peer from agents to own an
// unscoped replan obligation, using a hash of the canonical work key over the
// sorted registered-agent set. This is LoopX peer_v1 ownership precedence rule
// 4: unscoped replan obligations are assigned to exactly one peer by hashing a
// canonical work key over the sorted registered-agent set — deterministic, not
// persisted as rank. Registration order must not change the assignment, so the
// agent set is sorted (and de-duplicated) before hashing.
//
// Statelessness is the point: no leader election, no consensus, no persisted
// rank. The cost is that no peer can be preferred; an explicit claimed_by or
// active lease always wins over this fallback (see Kernel.Claim / AcquireLease).
func AssignPeer(workKey string, agents []string) (string, error) {
	canonical := canonicalAgents(agents)
	if len(canonical) == 0 {
		return "", errors.New("controlplane: no registered agents to assign")
	}
	if len(canonical) == 1 {
		return canonical[0], nil
	}
	h := fnv.New64a()
	h.Write([]byte(workKey))
	h.Write([]byte{0})
	h.Write([]byte(strings.Join(canonical, "\x00")))
	idx := int(h.Sum64() % uint64(len(canonical)))
	return canonical[idx], nil
}

// canonicalAgents sorts and de-duplicates the agent set so registration order
// and duplicates cannot change the assignment.
func canonicalAgents(agents []string) []string {
	if len(agents) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(agents))
	out := make([]string, 0, len(agents))
	for _, a := range agents {
		if a == "" {
			continue
		}
		if _, ok := seen[a]; ok {
			continue
		}
		seen[a] = struct{}{}
		out = append(out, a)
	}
	sort.Strings(out)
	return out
}

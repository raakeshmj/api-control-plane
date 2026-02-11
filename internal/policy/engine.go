package policy

import (
	"net/http"
	"strings"
	"sync"
)

// Matcher defines criteria to apply a policy
type Matcher struct {
	Method string `json:"method,omitempty"` // "*" or specific
	Path   string `json:"path"`             // Prefix match
}

// Rules defines what to enforce
type Rules struct {
	AuthRequired bool    `json:"auth_required"`
	RateLimit    float64 `json:"rate_limit"` // Requests per second
	Burst        int     `json:"burst"`
}

// Policy is a named set of rules
type Policy struct {
	ID      string  `json:"id"`
	Matcher Matcher `json:"matcher"`
	Rules   Rules   `json:"rules"`
}

// Engine evaluates requests against policies
type Engine struct {
	mu       sync.RWMutex
	policies []Policy
}

func NewEngine() *Engine {
	return &Engine{
		policies: []Policy{},
	}
}

// LoadPolicies replaces the current set
func (e *Engine) LoadPolicies(newPolicies []Policy) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.policies = newPolicies
}

// Evaluate finds the first matching policy
// Conflict Resolution: First Match Wins (ordered list).
// Production Note: Verify specificity sorting (Longest Path Match).
func (e *Engine) Evaluate(r *http.Request) *Policy {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for i := range e.policies {
		p := &e.policies[i]
		if match(p.Matcher, r) {
			return p
		}
	}
	return nil
}

func match(m Matcher, r *http.Request) bool {
	// Method Match
	if m.Method != "" && m.Method != "*" && m.Method != r.Method {
		return false
	}

	// Path Match (Prefix)
	if !strings.HasPrefix(r.URL.Path, m.Path) {
		return false
	}

	return true
}

package queuev2

import (
	"testing"

	fuzz "github.com/google/gofuzz"
	"github.com/stretchr/testify/assert"
)

func TestNot(t *testing.T) {
	tests := []struct {
		name     string
		input    Predicate
		expected Predicate
	}{
		{
			name:     "universalPredicate returns emptyPredicate",
			input:    NewUniversalPredicate(),
			expected: NewEmptyPredicate(),
		},
		{
			name:     "emptyPredicate returns universalPredicate",
			input:    NewEmptyPredicate(),
			expected: NewUniversalPredicate(),
		},
		{
			name:     "domainIDPredicate with isExclusive=true returns isExclusive=false",
			input:    NewDomainIDPredicate([]string{"domain1", "domain2"}, true),
			expected: NewDomainIDPredicate([]string{"domain1", "domain2"}, false),
		},
		{
			name:     "domainIDPredicate with isExclusive=false returns isExclusive=true",
			input:    NewDomainIDPredicate([]string{"domain1", "domain2"}, false),
			expected: NewDomainIDPredicate([]string{"domain1", "domain2"}, true),
		},
		{
			name:     "domainIDPredicate with empty domains and isExclusive=true",
			input:    NewDomainIDPredicate([]string{}, true),
			expected: NewDomainIDPredicate([]string{}, false),
		},
		{
			name:     "domainIDPredicate with empty domains and isExclusive=false",
			input:    NewDomainIDPredicate([]string{}, false),
			expected: NewUniversalPredicate(),
		},
		{
			name:     "domainIDPredicate with single domain",
			input:    NewDomainIDPredicate([]string{"single-domain"}, true),
			expected: NewDomainIDPredicate([]string{"single-domain"}, false),
		},
		{
			name:     "domainIDPredicate with multiple domains",
			input:    NewDomainIDPredicate([]string{"domain1", "domain2", "domain3", "domain4"}, false),
			expected: NewDomainIDPredicate([]string{"domain1", "domain2", "domain3", "domain4"}, true),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Not(tt.input)
			assert.NotNil(t, result)

			// Use the Equals method to compare predicates
			assert.True(t, tt.expected.Equals(result),
				"expected %+v, got %+v", tt.expected, result)
		})
	}
}

func TestAnd(t *testing.T) {
	tests := []struct {
		name     string
		p1       Predicate
		p2       Predicate
		expected Predicate
	}{
		// universalPredicate cases
		{
			name:     "universalPredicate AND universalPredicate",
			p1:       NewUniversalPredicate(),
			p2:       NewUniversalPredicate(),
			expected: NewUniversalPredicate(),
		},
		{
			name:     "universalPredicate AND emptyPredicate",
			p1:       NewUniversalPredicate(),
			p2:       NewEmptyPredicate(),
			expected: NewEmptyPredicate(),
		},
		{
			name:     "universalPredicate AND domainIDPredicate",
			p1:       NewUniversalPredicate(),
			p2:       NewDomainIDPredicate([]string{"domain1", "domain2"}, false),
			expected: NewDomainIDPredicate([]string{"domain1", "domain2"}, false),
		},

		// emptyPredicate cases
		{
			name:     "emptyPredicate AND universalPredicate",
			p1:       NewEmptyPredicate(),
			p2:       NewUniversalPredicate(),
			expected: NewEmptyPredicate(),
		},
		{
			name:     "emptyPredicate AND emptyPredicate",
			p1:       NewEmptyPredicate(),
			p2:       NewEmptyPredicate(),
			expected: NewEmptyPredicate(),
		},
		{
			name:     "emptyPredicate AND domainIDPredicate",
			p1:       NewEmptyPredicate(),
			p2:       NewDomainIDPredicate([]string{"domain1"}, true),
			expected: NewEmptyPredicate(),
		},

		// domainIDPredicate AND universalPredicate
		{
			name:     "domainIDPredicate AND universalPredicate",
			p1:       NewDomainIDPredicate([]string{"domain1", "domain2"}, true),
			p2:       NewUniversalPredicate(),
			expected: NewDomainIDPredicate([]string{"domain1", "domain2"}, true),
		},

		// domainIDPredicate AND emptyPredicate
		{
			name:     "domainIDPredicate AND emptyPredicate",
			p1:       NewDomainIDPredicate([]string{"domain1", "domain2"}, false),
			p2:       NewEmptyPredicate(),
			expected: NewEmptyPredicate(),
		},

		// domainIDPredicate AND domainIDPredicate cases
		{
			name:     "exclusive AND exclusive - union domains",
			p1:       NewDomainIDPredicate([]string{"domain1", "domain2"}, true),
			p2:       NewDomainIDPredicate([]string{"domain3", "domain4"}, true),
			expected: NewDomainIDPredicate([]string{"domain1", "domain2", "domain3", "domain4"}, true),
		},
		{
			name:     "exclusive AND exclusive - overlapping domains",
			p1:       NewDomainIDPredicate([]string{"domain1", "domain2"}, true),
			p2:       NewDomainIDPredicate([]string{"domain2", "domain3"}, true),
			expected: NewDomainIDPredicate([]string{"domain1", "domain2", "domain3"}, true),
		},
		{
			name:     "exclusive AND inclusive - p2 domains not in p1",
			p1:       NewDomainIDPredicate([]string{"domain1", "domain2"}, true),
			p2:       NewDomainIDPredicate([]string{"domain2", "domain3", "domain4"}, false),
			expected: NewDomainIDPredicate([]string{"domain3", "domain4"}, false),
		},
		{
			name:     "exclusive AND inclusive - all p2 domains in p1",
			p1:       NewDomainIDPredicate([]string{"domain1", "domain2", "domain3"}, true),
			p2:       NewDomainIDPredicate([]string{"domain1", "domain2"}, false),
			expected: NewEmptyPredicate(),
		},
		{
			name:     "inclusive AND exclusive - p1 domains not in p2",
			p1:       NewDomainIDPredicate([]string{"domain1", "domain2", "domain3"}, false),
			p2:       NewDomainIDPredicate([]string{"domain2", "domain4"}, true),
			expected: NewDomainIDPredicate([]string{"domain1", "domain3"}, false),
		},
		{
			name:     "inclusive AND exclusive - all p1 domains in p2",
			p1:       NewDomainIDPredicate([]string{"domain1", "domain2"}, false),
			p2:       NewDomainIDPredicate([]string{"domain1", "domain2", "domain3"}, true),
			expected: NewEmptyPredicate(),
		},
		{
			name:     "inclusive AND inclusive - intersection",
			p1:       NewDomainIDPredicate([]string{"domain1", "domain2", "domain3"}, false),
			p2:       NewDomainIDPredicate([]string{"domain2", "domain3", "domain4"}, false),
			expected: NewDomainIDPredicate([]string{"domain2", "domain3"}, false),
		},
		{
			name:     "inclusive AND inclusive - no intersection",
			p1:       NewDomainIDPredicate([]string{"domain1", "domain2"}, false),
			p2:       NewDomainIDPredicate([]string{"domain3", "domain4"}, false),
			expected: NewEmptyPredicate(),
		},
		{
			name:     "inclusive AND inclusive - same domains",
			p1:       NewDomainIDPredicate([]string{"domain1", "domain2"}, false),
			p2:       NewDomainIDPredicate([]string{"domain1", "domain2"}, false),
			expected: NewDomainIDPredicate([]string{"domain1", "domain2"}, false),
		},

		// Edge cases with empty domain lists
		{
			name:     "empty exclusive AND non-empty exclusive",
			p1:       NewDomainIDPredicate([]string{}, true),
			p2:       NewDomainIDPredicate([]string{"domain1"}, true),
			expected: NewDomainIDPredicate([]string{"domain1"}, true),
		},
		{
			name:     "empty inclusive AND non-empty inclusive",
			p1:       NewDomainIDPredicate([]string{}, false),
			p2:       NewDomainIDPredicate([]string{"domain1"}, false),
			expected: NewEmptyPredicate(),
		},
		{
			name:     "empty exclusive AND non-empty inclusive",
			p1:       NewDomainIDPredicate([]string{}, true),
			p2:       NewDomainIDPredicate([]string{"domain1"}, false),
			expected: NewDomainIDPredicate([]string{"domain1"}, false),
		},
		{
			name:     "non-empty inclusive AND empty exclusive",
			p1:       NewDomainIDPredicate([]string{"domain1"}, false),
			p2:       NewDomainIDPredicate([]string{}, true),
			expected: NewDomainIDPredicate([]string{"domain1"}, false),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := And(tt.p1, tt.p2)
			assert.NotNil(t, result)

			// Use the Equals method to compare predicates
			assert.True(t, tt.expected.Equals(result),
				"expected %+v, got %+v", tt.expected, result)
		})
	}
}

func predicateOperationFuzzGenerator(p *Predicate, c fuzz.Continue) {
	switch c.Intn(3) {
	case 0:
		*p = NewUniversalPredicate()
	case 1:
		*p = NewEmptyPredicate()
	case 2:
		var domainIDPredicate domainIDPredicate
		c.Fuzz(&domainIDPredicate)
		*p = &domainIDPredicate
	default:
		panic("invalid predicate type")
	}
}

func TestAnd_Commutativity(t *testing.T) {
	f := fuzz.New().Funcs(predicateOperationFuzzGenerator)
	for i := 0; i < 1000; i++ {
		var p1, p2 Predicate
		f.Fuzz(&p1)
		f.Fuzz(&p2)

		result1 := And(p1, p2)
		result2 := And(p2, p1)

		assert.True(t, result1.Equals(result2),
			"And should be commutative: And(p1, p2) should equal And(p2, p1)")
	}
}

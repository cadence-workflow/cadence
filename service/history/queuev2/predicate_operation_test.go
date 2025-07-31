package queuev2

import (
	"testing"

	fuzz "github.com/google/gofuzz"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/persistence"
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

func TestAnd_LogicalCorrectness(t *testing.T) {
	ctrl := gomock.NewController(t)

	// Test data with different domain IDs
	testTasks := []*persistence.MockTask{
		// Task with domain1
		func() *persistence.MockTask {
			task := persistence.NewMockTask(ctrl)
			task.EXPECT().GetDomainID().Return("domain1").AnyTimes()
			return task
		}(),
		// Task with domain2
		func() *persistence.MockTask {
			task := persistence.NewMockTask(ctrl)
			task.EXPECT().GetDomainID().Return("domain2").AnyTimes()
			return task
		}(),
		// Task with domain3
		func() *persistence.MockTask {
			task := persistence.NewMockTask(ctrl)
			task.EXPECT().GetDomainID().Return("domain3").AnyTimes()
			return task
		}(),
		// Task with domain4
		func() *persistence.MockTask {
			task := persistence.NewMockTask(ctrl)
			task.EXPECT().GetDomainID().Return("domain4").AnyTimes()
			return task
		}(),
	}

	// Test cases with different predicate combinations
	testCases := []struct {
		name string
		p1   Predicate
		p2   Predicate
	}{
		// Universal and Empty predicates
		{
			name: "universal AND universal",
			p1:   NewUniversalPredicate(),
			p2:   NewUniversalPredicate(),
		},
		{
			name: "universal AND empty",
			p1:   NewUniversalPredicate(),
			p2:   NewEmptyPredicate(),
		},
		{
			name: "empty AND universal",
			p1:   NewEmptyPredicate(),
			p2:   NewUniversalPredicate(),
		},
		{
			name: "empty AND empty",
			p1:   NewEmptyPredicate(),
			p2:   NewEmptyPredicate(),
		},

		// Universal with domain predicates
		{
			name: "universal AND domain inclusive",
			p1:   NewUniversalPredicate(),
			p2:   NewDomainIDPredicate([]string{"domain1", "domain2"}, false),
		},
		{
			name: "universal AND domain exclusive",
			p1:   NewUniversalPredicate(),
			p2:   NewDomainIDPredicate([]string{"domain1", "domain2"}, true),
		},
		{
			name: "domain inclusive AND universal",
			p1:   NewDomainIDPredicate([]string{"domain1", "domain3"}, false),
			p2:   NewUniversalPredicate(),
		},
		{
			name: "domain exclusive AND universal",
			p1:   NewDomainIDPredicate([]string{"domain1", "domain3"}, true),
			p2:   NewUniversalPredicate(),
		},

		// Empty with domain predicates
		{
			name: "empty AND domain inclusive",
			p1:   NewEmptyPredicate(),
			p2:   NewDomainIDPredicate([]string{"domain1", "domain2"}, false),
		},
		{
			name: "empty AND domain exclusive",
			p1:   NewEmptyPredicate(),
			p2:   NewDomainIDPredicate([]string{"domain1", "domain2"}, true),
		},
		{
			name: "domain inclusive AND empty",
			p1:   NewDomainIDPredicate([]string{"domain2", "domain4"}, false),
			p2:   NewEmptyPredicate(),
		},
		{
			name: "domain exclusive AND empty",
			p1:   NewDomainIDPredicate([]string{"domain2", "domain4"}, true),
			p2:   NewEmptyPredicate(),
		},

		// Domain predicate combinations
		{
			name: "inclusive AND inclusive - overlapping",
			p1:   NewDomainIDPredicate([]string{"domain1", "domain2"}, false),
			p2:   NewDomainIDPredicate([]string{"domain2", "domain3"}, false),
		},
		{
			name: "inclusive AND inclusive - disjoint",
			p1:   NewDomainIDPredicate([]string{"domain1", "domain2"}, false),
			p2:   NewDomainIDPredicate([]string{"domain3", "domain4"}, false),
		},
		{
			name: "exclusive AND exclusive - overlapping",
			p1:   NewDomainIDPredicate([]string{"domain1", "domain2"}, true),
			p2:   NewDomainIDPredicate([]string{"domain2", "domain3"}, true),
		},
		{
			name: "exclusive AND exclusive - disjoint",
			p1:   NewDomainIDPredicate([]string{"domain1"}, true),
			p2:   NewDomainIDPredicate([]string{"domain3"}, true),
		},
		{
			name: "inclusive AND exclusive - overlapping",
			p1:   NewDomainIDPredicate([]string{"domain1", "domain2", "domain3"}, false),
			p2:   NewDomainIDPredicate([]string{"domain2", "domain4"}, true),
		},
		{
			name: "exclusive AND inclusive - overlapping",
			p1:   NewDomainIDPredicate([]string{"domain1", "domain3"}, true),
			p2:   NewDomainIDPredicate([]string{"domain2", "domain3", "domain4"}, false),
		},

		// Edge cases with empty domain lists
		{
			name: "empty inclusive AND non-empty inclusive",
			p1:   NewDomainIDPredicate([]string{}, false),
			p2:   NewDomainIDPredicate([]string{"domain1"}, false),
		},
		{
			name: "empty exclusive AND non-empty exclusive",
			p1:   NewDomainIDPredicate([]string{}, true),
			p2:   NewDomainIDPredicate([]string{"domain1"}, true),
		},
		{
			name: "empty inclusive AND empty exclusive",
			p1:   NewDomainIDPredicate([]string{}, false),
			p2:   NewDomainIDPredicate([]string{}, true),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			andPredicate := And(tc.p1, tc.p2)

			// Test the logical property for each task
			for i, task := range testTasks {
				p1Result := tc.p1.Check(task)
				p2Result := tc.p2.Check(task)
				expectedResult := p1Result && p2Result
				actualResult := andPredicate.Check(task)

				assert.Equal(t, expectedResult, actualResult,
					"For task %d (domain=%s): And(p1, p2).Check(task) should equal p1.Check(task) && p2.Check(task). "+
						"p1.Check(task)=%t, p2.Check(task)=%t, expected=%t, actual=%t",
					i, task.GetDomainID(), p1Result, p2Result, expectedResult, actualResult)
			}
		})
	}
}

func TestAnd_LogicalCorrectness_FuzzTesting(t *testing.T) {
	ctrl := gomock.NewController(t)

	// Create a variety of test tasks with different domain IDs
	domains := []string{"domain1", "domain2", "domain3", "domain4", "domain5", ""}
	testTasks := make([]*persistence.MockTask, len(domains))
	for i, domain := range domains {
		task := persistence.NewMockTask(ctrl)
		task.EXPECT().GetDomainID().Return(domain).AnyTimes()
		testTasks[i] = task
	}

	f := fuzz.New().Funcs(predicateOperationFuzzGenerator)
	for i := 0; i < 500; i++ {
		var p1, p2 Predicate
		f.Fuzz(&p1)
		f.Fuzz(&p2)

		andPredicate := And(p1, p2)

		// Test the logical property for each task
		for _, task := range testTasks {
			p1Result := p1.Check(task)
			p2Result := p2.Check(task)
			expectedResult := p1Result && p2Result
			actualResult := andPredicate.Check(task)

			assert.Equal(t, expectedResult, actualResult,
				"Fuzz test iteration %d: And(p1, p2).Check(task) should equal p1.Check(task) && p2.Check(task). "+
					"Task domain: %s, p1.Check(task)=%t, p2.Check(task)=%t, expected=%t, actual=%t, "+
					"p1=%+v, p2=%+v",
				i, task.GetDomainID(), p1Result, p2Result, expectedResult, actualResult, p1, p2)
		}
	}
}

func TestOr(t *testing.T) {
	tests := []struct {
		name     string
		p1       Predicate
		p2       Predicate
		expected Predicate
	}{
		// universalPredicate cases
		{
			name:     "universalPredicate OR universalPredicate",
			p1:       NewUniversalPredicate(),
			p2:       NewUniversalPredicate(),
			expected: NewUniversalPredicate(),
		},
		{
			name:     "universalPredicate OR emptyPredicate",
			p1:       NewUniversalPredicate(),
			p2:       NewEmptyPredicate(),
			expected: NewUniversalPredicate(),
		},
		{
			name:     "universalPredicate OR domainIDPredicate",
			p1:       NewUniversalPredicate(),
			p2:       NewDomainIDPredicate([]string{"domain1", "domain2"}, false),
			expected: NewUniversalPredicate(),
		},

		// emptyPredicate cases
		{
			name:     "emptyPredicate OR universalPredicate",
			p1:       NewEmptyPredicate(),
			p2:       NewUniversalPredicate(),
			expected: NewUniversalPredicate(),
		},
		{
			name:     "emptyPredicate OR emptyPredicate",
			p1:       NewEmptyPredicate(),
			p2:       NewEmptyPredicate(),
			expected: NewEmptyPredicate(),
		},
		{
			name:     "emptyPredicate OR domainIDPredicate",
			p1:       NewEmptyPredicate(),
			p2:       NewDomainIDPredicate([]string{"domain1"}, true),
			expected: NewDomainIDPredicate([]string{"domain1"}, true),
		},

		// domainIDPredicate OR universalPredicate
		{
			name:     "domainIDPredicate OR universalPredicate",
			p1:       NewDomainIDPredicate([]string{"domain1", "domain2"}, true),
			p2:       NewUniversalPredicate(),
			expected: NewUniversalPredicate(),
		},

		// domainIDPredicate OR emptyPredicate
		{
			name:     "domainIDPredicate OR emptyPredicate",
			p1:       NewDomainIDPredicate([]string{"domain1", "domain2"}, false),
			p2:       NewEmptyPredicate(),
			expected: NewDomainIDPredicate([]string{"domain1", "domain2"}, false),
		},

		// domainIDPredicate OR domainIDPredicate cases
		{
			name:     "exclusive OR exclusive - intersection exists",
			p1:       NewDomainIDPredicate([]string{"domain1", "domain2"}, true),
			p2:       NewDomainIDPredicate([]string{"domain2", "domain3"}, true),
			expected: NewDomainIDPredicate([]string{"domain2"}, true),
		},
		{
			name:     "exclusive OR exclusive - no intersection",
			p1:       NewDomainIDPredicate([]string{"domain1", "domain2"}, true),
			p2:       NewDomainIDPredicate([]string{"domain3", "domain4"}, true),
			expected: NewUniversalPredicate(),
		},
		{
			name:     "exclusive OR inclusive - p1 excludes some of p2",
			p1:       NewDomainIDPredicate([]string{"domain1", "domain2"}, true),
			p2:       NewDomainIDPredicate([]string{"domain2", "domain3", "domain4"}, false),
			expected: NewDomainIDPredicate([]string{"domain1"}, true),
		},
		{
			name:     "exclusive OR inclusive - p1 excludes all of p2",
			p1:       NewDomainIDPredicate([]string{"domain1", "domain2", "domain3"}, true),
			p2:       NewDomainIDPredicate([]string{"domain1", "domain2"}, false),
			expected: NewDomainIDPredicate([]string{"domain3"}, true),
		},
		{
			name:     "exclusive OR inclusive - p1 excludes none of p2",
			p1:       NewDomainIDPredicate([]string{"domain1"}, true),
			p2:       NewDomainIDPredicate([]string{"domain2", "domain3"}, false),
			expected: NewDomainIDPredicate([]string{"domain1"}, true),
		},
		{
			name:     "inclusive OR exclusive - p2 excludes some of p1",
			p1:       NewDomainIDPredicate([]string{"domain1", "domain2", "domain3"}, false),
			p2:       NewDomainIDPredicate([]string{"domain2", "domain4"}, true),
			expected: NewDomainIDPredicate([]string{"domain4"}, true),
		},
		{
			name:     "inclusive OR exclusive - p2 excludes all of p1",
			p1:       NewDomainIDPredicate([]string{"domain1", "domain2"}, false),
			p2:       NewDomainIDPredicate([]string{"domain1", "domain2", "domain3"}, true),
			expected: NewDomainIDPredicate([]string{"domain3"}, true),
		},
		{
			name:     "inclusive OR exclusive - p2 excludes none of p1",
			p1:       NewDomainIDPredicate([]string{"domain1", "domain2"}, false),
			p2:       NewDomainIDPredicate([]string{"domain3"}, true),
			expected: NewDomainIDPredicate([]string{"domain3"}, true),
		},
		{
			name:     "inclusive OR inclusive - union",
			p1:       NewDomainIDPredicate([]string{"domain1", "domain2"}, false),
			p2:       NewDomainIDPredicate([]string{"domain2", "domain3", "domain4"}, false),
			expected: NewDomainIDPredicate([]string{"domain1", "domain2", "domain3", "domain4"}, false),
		},
		{
			name:     "inclusive OR inclusive - same domains",
			p1:       NewDomainIDPredicate([]string{"domain1", "domain2"}, false),
			p2:       NewDomainIDPredicate([]string{"domain1", "domain2"}, false),
			expected: NewDomainIDPredicate([]string{"domain1", "domain2"}, false),
		},

		// Edge cases with empty domain lists
		{
			name:     "empty exclusive OR non-empty exclusive",
			p1:       NewDomainIDPredicate([]string{}, true),
			p2:       NewDomainIDPredicate([]string{"domain1"}, true),
			expected: NewUniversalPredicate(),
		},
		{
			name:     "empty inclusive OR non-empty inclusive",
			p1:       NewDomainIDPredicate([]string{}, false),
			p2:       NewDomainIDPredicate([]string{"domain1"}, false),
			expected: NewDomainIDPredicate([]string{"domain1"}, false),
		},
		{
			name:     "empty exclusive OR non-empty inclusive",
			p1:       NewDomainIDPredicate([]string{}, true),
			p2:       NewDomainIDPredicate([]string{"domain1"}, false),
			expected: NewUniversalPredicate(),
		},
		{
			name:     "non-empty inclusive OR empty exclusive",
			p1:       NewDomainIDPredicate([]string{"domain1"}, false),
			p2:       NewDomainIDPredicate([]string{}, true),
			expected: NewUniversalPredicate(),
		},
		{
			name:     "empty exclusive OR empty exclusive",
			p1:       NewDomainIDPredicate([]string{}, true),
			p2:       NewDomainIDPredicate([]string{}, true),
			expected: NewUniversalPredicate(),
		},
		{
			name:     "empty inclusive OR empty inclusive",
			p1:       NewDomainIDPredicate([]string{}, false),
			p2:       NewDomainIDPredicate([]string{}, false),
			expected: NewEmptyPredicate(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Or(tt.p1, tt.p2)
			assert.NotNil(t, result)

			// Use the Equals method to compare predicates
			assert.True(t, tt.expected.Equals(result),
				"expected %+v, got %+v", tt.expected, result)
		})
	}
}

func TestOr_Commutativity(t *testing.T) {
	f := fuzz.New().Funcs(predicateOperationFuzzGenerator)
	for i := 0; i < 1000; i++ {
		var p1, p2 Predicate
		f.Fuzz(&p1)
		f.Fuzz(&p2)

		result1 := Or(p1, p2)
		result2 := Or(p2, p1)

		assert.True(t, result1.Equals(result2),
			"Or should be commutative: Or(p1, p2) should equal Or(p2, p1)")
	}
}

func TestOr_LogicalCorrectness(t *testing.T) {
	ctrl := gomock.NewController(t)

	// Test data with different domain IDs
	testTasks := []*persistence.MockTask{
		// Task with domain1
		func() *persistence.MockTask {
			task := persistence.NewMockTask(ctrl)
			task.EXPECT().GetDomainID().Return("domain1").AnyTimes()
			return task
		}(),
		// Task with domain2
		func() *persistence.MockTask {
			task := persistence.NewMockTask(ctrl)
			task.EXPECT().GetDomainID().Return("domain2").AnyTimes()
			return task
		}(),
		// Task with domain3
		func() *persistence.MockTask {
			task := persistence.NewMockTask(ctrl)
			task.EXPECT().GetDomainID().Return("domain3").AnyTimes()
			return task
		}(),
		// Task with domain4
		func() *persistence.MockTask {
			task := persistence.NewMockTask(ctrl)
			task.EXPECT().GetDomainID().Return("domain4").AnyTimes()
			return task
		}(),
	}

	// Test cases with different predicate combinations
	testCases := []struct {
		name string
		p1   Predicate
		p2   Predicate
	}{
		// Universal and Empty predicates
		{
			name: "universal OR universal",
			p1:   NewUniversalPredicate(),
			p2:   NewUniversalPredicate(),
		},
		{
			name: "universal OR empty",
			p1:   NewUniversalPredicate(),
			p2:   NewEmptyPredicate(),
		},
		{
			name: "empty OR universal",
			p1:   NewEmptyPredicate(),
			p2:   NewUniversalPredicate(),
		},
		{
			name: "empty OR empty",
			p1:   NewEmptyPredicate(),
			p2:   NewEmptyPredicate(),
		},

		// Universal with domain predicates
		{
			name: "universal OR domain inclusive",
			p1:   NewUniversalPredicate(),
			p2:   NewDomainIDPredicate([]string{"domain1", "domain2"}, false),
		},
		{
			name: "universal OR domain exclusive",
			p1:   NewUniversalPredicate(),
			p2:   NewDomainIDPredicate([]string{"domain1", "domain2"}, true),
		},
		{
			name: "domain inclusive OR universal",
			p1:   NewDomainIDPredicate([]string{"domain1", "domain3"}, false),
			p2:   NewUniversalPredicate(),
		},
		{
			name: "domain exclusive OR universal",
			p1:   NewDomainIDPredicate([]string{"domain1", "domain3"}, true),
			p2:   NewUniversalPredicate(),
		},

		// Empty with domain predicates
		{
			name: "empty OR domain inclusive",
			p1:   NewEmptyPredicate(),
			p2:   NewDomainIDPredicate([]string{"domain1", "domain2"}, false),
		},
		{
			name: "empty OR domain exclusive",
			p1:   NewEmptyPredicate(),
			p2:   NewDomainIDPredicate([]string{"domain1", "domain2"}, true),
		},
		{
			name: "domain inclusive OR empty",
			p1:   NewDomainIDPredicate([]string{"domain2", "domain4"}, false),
			p2:   NewEmptyPredicate(),
		},
		{
			name: "domain exclusive OR empty",
			p1:   NewDomainIDPredicate([]string{"domain2", "domain4"}, true),
			p2:   NewEmptyPredicate(),
		},

		// Domain predicate combinations
		{
			name: "inclusive OR inclusive - overlapping",
			p1:   NewDomainIDPredicate([]string{"domain1", "domain2"}, false),
			p2:   NewDomainIDPredicate([]string{"domain2", "domain3"}, false),
		},
		{
			name: "inclusive OR inclusive - disjoint",
			p1:   NewDomainIDPredicate([]string{"domain1", "domain2"}, false),
			p2:   NewDomainIDPredicate([]string{"domain3", "domain4"}, false),
		},
		{
			name: "exclusive OR exclusive - overlapping",
			p1:   NewDomainIDPredicate([]string{"domain1", "domain2"}, true),
			p2:   NewDomainIDPredicate([]string{"domain2", "domain3"}, true),
		},
		{
			name: "exclusive OR exclusive - disjoint",
			p1:   NewDomainIDPredicate([]string{"domain1"}, true),
			p2:   NewDomainIDPredicate([]string{"domain3"}, true),
		},
		{
			name: "inclusive OR exclusive - overlapping",
			p1:   NewDomainIDPredicate([]string{"domain1", "domain2", "domain3"}, false),
			p2:   NewDomainIDPredicate([]string{"domain2", "domain4"}, true),
		},
		{
			name: "exclusive OR inclusive - overlapping",
			p1:   NewDomainIDPredicate([]string{"domain1", "domain3"}, true),
			p2:   NewDomainIDPredicate([]string{"domain2", "domain3", "domain4"}, false),
		},

		// Edge cases with empty domain lists
		{
			name: "empty inclusive OR non-empty inclusive",
			p1:   NewDomainIDPredicate([]string{}, false),
			p2:   NewDomainIDPredicate([]string{"domain1"}, false),
		},
		{
			name: "empty exclusive OR non-empty exclusive",
			p1:   NewDomainIDPredicate([]string{}, true),
			p2:   NewDomainIDPredicate([]string{"domain1"}, true),
		},
		{
			name: "empty inclusive OR empty exclusive",
			p1:   NewDomainIDPredicate([]string{}, false),
			p2:   NewDomainIDPredicate([]string{}, true),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			orPredicate := Or(tc.p1, tc.p2)

			// Test the logical property for each task
			for i, task := range testTasks {
				p1Result := tc.p1.Check(task)
				p2Result := tc.p2.Check(task)
				expectedResult := p1Result || p2Result
				actualResult := orPredicate.Check(task)

				assert.Equal(t, expectedResult, actualResult,
					"For task %d (domain=%s): Or(p1, p2).Check(task) should equal p1.Check(task) || p2.Check(task). "+
						"p1.Check(task)=%t, p2.Check(task)=%t, expected=%t, actual=%t",
					i, task.GetDomainID(), p1Result, p2Result, expectedResult, actualResult)
			}
		})
	}
}

func TestOr_LogicalCorrectness_FuzzTesting(t *testing.T) {
	ctrl := gomock.NewController(t)

	// Create a variety of test tasks with different domain IDs
	domains := []string{"domain1", "domain2", "domain3", "domain4", "domain5", ""}
	testTasks := make([]*persistence.MockTask, len(domains))
	for i, domain := range domains {
		task := persistence.NewMockTask(ctrl)
		task.EXPECT().GetDomainID().Return(domain).AnyTimes()
		testTasks[i] = task
	}

	f := fuzz.New().Funcs(predicateOperationFuzzGenerator)
	for i := 0; i < 500; i++ {
		var p1, p2 Predicate
		f.Fuzz(&p1)
		f.Fuzz(&p2)

		orPredicate := Or(p1, p2)

		// Test the logical property for each task
		for _, task := range testTasks {
			p1Result := p1.Check(task)
			p2Result := p2.Check(task)
			expectedResult := p1Result || p2Result
			actualResult := orPredicate.Check(task)

			assert.Equal(t, expectedResult, actualResult,
				"Fuzz test iteration %d: Or(p1, p2).Check(task) should equal p1.Check(task) || p2.Check(task). "+
					"Task domain: %s, p1.Check(task)=%t, p2.Check(task)=%t, expected=%t, actual=%t, "+
					"p1=%+v, p2=%+v",
				i, task.GetDomainID(), p1Result, p2Result, expectedResult, actualResult, p1, p2)
		}
	}
}

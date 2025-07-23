package thrift

import (
	"github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/common/types"
)

func FromPredicateType(t types.PredicateType) *shared.PredicateType {
	v := shared.PredicateTypeUniversal
	switch t {
	case types.PredicateTypeUniversal:
		return &v
	case types.PredicateTypeEmpty:
		v = shared.PredicateTypeEmpty
		return &v
	case types.PredicateTypeAnd:
		v = shared.PredicateTypeAnd
		return &v
	case types.PredicateTypeOr:
		v = shared.PredicateTypeOr
		return &v
	case types.PredicateTypeNot:
		v = shared.PredicateTypeNot
		return &v
	case types.PredicateTypeDomainID:
		v = shared.PredicateTypeDomainID
		return &v
	}
	// default to universal
	return &v
}

func ToPredicateType(t *shared.PredicateType) types.PredicateType {
	if t == nil {
		return types.PredicateTypeUniversal
	}
	switch *t {
	case shared.PredicateTypeUniversal:
		return types.PredicateTypeUniversal
	case shared.PredicateTypeEmpty:
		return types.PredicateTypeEmpty
	case shared.PredicateTypeAnd:
		return types.PredicateTypeAnd
	case shared.PredicateTypeOr:
		return types.PredicateTypeOr
	case shared.PredicateTypeNot:
		return types.PredicateTypeNot
	case shared.PredicateTypeDomainID:
		return types.PredicateTypeDomainID
	}
	// default to universal
	return types.PredicateTypeUniversal
}

func FromUniversalPredicateAttributes(a *types.UniversalPredicateAttributes) *shared.UniversalPredicateAttributes {
	if a == nil {
		return nil
	}
	return &shared.UniversalPredicateAttributes{}
}

func ToUniversalPredicateAttributes(a *shared.UniversalPredicateAttributes) *types.UniversalPredicateAttributes {
	if a == nil {
		return nil
	}
	return &types.UniversalPredicateAttributes{}
}

func FromEmptyPredicateAttributes(a *types.EmptyPredicateAttributes) *shared.EmptyPredicateAttributes {
	if a == nil {
		return nil
	}
	return &shared.EmptyPredicateAttributes{}
}

func ToEmptyPredicateAttributes(a *shared.EmptyPredicateAttributes) *types.EmptyPredicateAttributes {
	if a == nil {
		return nil
	}
	return &types.EmptyPredicateAttributes{}
}

func FromAndPredicateAttributes(a *types.AndPredicateAttributes) *shared.AndPredicateAttributes {
	if a == nil {
		return nil
	}
	return &shared.AndPredicateAttributes{
		Predicates: FromPredicateList(a.Predicates),
	}
}

func ToAndPredicateAttributes(a *shared.AndPredicateAttributes) *types.AndPredicateAttributes {
	if a == nil {
		return nil
	}
	return &types.AndPredicateAttributes{
		Predicates: ToPredicateList(a.Predicates),
	}
}

func FromOrPredicateAttributes(a *types.OrPredicateAttributes) *shared.OrPredicateAttributes {
	if a == nil {
		return nil
	}
	return &shared.OrPredicateAttributes{
		Predicates: FromPredicateList(a.Predicates),
	}
}

func ToOrPredicateAttributes(a *shared.OrPredicateAttributes) *types.OrPredicateAttributes {
	if a == nil {
		return nil
	}
	return &types.OrPredicateAttributes{
		Predicates: ToPredicateList(a.Predicates),
	}
}

func FromNotPredicateAttributes(a *types.NotPredicateAttributes) *shared.NotPredicateAttributes {
	if a == nil {
		return nil
	}
	return &shared.NotPredicateAttributes{
		Predicate: FromPredicate(a.Predicate),
	}
}

func ToNotPredicateAttributes(a *shared.NotPredicateAttributes) *types.NotPredicateAttributes {
	if a == nil {
		return nil
	}
	return &types.NotPredicateAttributes{
		Predicate: ToPredicate(a.Predicate),
	}
}

func FromDomainIDPredicateAttributes(a *types.DomainIDPredicateAttributes) *shared.DomainIDPredicateAttributes {
	if a == nil {
		return nil
	}
	return &shared.DomainIDPredicateAttributes{
		DomainIDs:   a.DomainIDs,
		IsExclusive: a.IsExclusive,
	}
}

func ToDomainIDPredicateAttributes(a *shared.DomainIDPredicateAttributes) *types.DomainIDPredicateAttributes {
	if a == nil {
		return nil
	}
	return &types.DomainIDPredicateAttributes{
		DomainIDs:   a.DomainIDs,
		IsExclusive: a.IsExclusive,
	}
}

func FromPredicate(p *types.Predicate) *shared.Predicate {
	if p == nil {
		return nil
	}
	return &shared.Predicate{
		PredicateType: FromPredicateType(p.PredicateType),
		UniversalPredicateAttributes: FromUniversalPredicateAttributes(p.UniversalPredicateAttributes),
		EmptyPredicateAttributes: FromEmptyPredicateAttributes(p.EmptyPredicateAttributes),
		AndPredicateAttributes: FromAndPredicateAttributes(p.AndPredicateAttributes),
		OrPredicateAttributes: FromOrPredicateAttributes(p.OrPredicateAttributes),
		NotPredicateAttributes: FromNotPredicateAttributes(p.NotPredicateAttributes),
		DomainIDPredicateAttributes: FromDomainIDPredicateAttributes(p.DomainIDPredicateAttributes),
	}
}

func ToPredicate(p *shared.Predicate) *types.Predicate {
	if p == nil {
		return nil
	}
	return &types.Predicate{
		PredicateType: ToPredicateType(p.PredicateType),
		UniversalPredicateAttributes: ToUniversalPredicateAttributes(p.UniversalPredicateAttributes),
		EmptyPredicateAttributes: ToEmptyPredicateAttributes(p.EmptyPredicateAttributes),
		AndPredicateAttributes: ToAndPredicateAttributes(p.AndPredicateAttributes),
		OrPredicateAttributes: ToOrPredicateAttributes(p.OrPredicateAttributes),
		NotPredicateAttributes: ToNotPredicateAttributes(p.NotPredicateAttributes),
		DomainIDPredicateAttributes: ToDomainIDPredicateAttributes(p.DomainIDPredicateAttributes),
	}
}

func FromPredicateList(l []*types.Predicate) []*shared.Predicate {
	if l == nil {
		return nil
	}
	predicates := make([]*shared.Predicate, len(l))
	for i, p := range l {
		predicates[i] = FromPredicate(p)
	}
	return predicates
}

func ToPredicateList(l []*shared.Predicate) []*types.Predicate {
	if l == nil {
		return nil
	}
	predicates := make([]*types.Predicate, len(l))
	for i, p := range l {
		predicates[i] = ToPredicate(p)
	}
	return predicates
}

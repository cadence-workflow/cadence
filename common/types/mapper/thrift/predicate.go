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
		PredicateType:                FromPredicateType(p.PredicateType),
		UniversalPredicateAttributes: FromUniversalPredicateAttributes(p.UniversalPredicateAttributes),
		EmptyPredicateAttributes:     FromEmptyPredicateAttributes(p.EmptyPredicateAttributes),
		DomainIDPredicateAttributes:  FromDomainIDPredicateAttributes(p.DomainIDPredicateAttributes),
	}
}

func ToPredicate(p *shared.Predicate) *types.Predicate {
	if p == nil {
		return nil
	}
	return &types.Predicate{
		PredicateType:                ToPredicateType(p.PredicateType),
		UniversalPredicateAttributes: ToUniversalPredicateAttributes(p.UniversalPredicateAttributes),
		EmptyPredicateAttributes:     ToEmptyPredicateAttributes(p.EmptyPredicateAttributes),
		DomainIDPredicateAttributes:  ToDomainIDPredicateAttributes(p.DomainIDPredicateAttributes),
	}
}

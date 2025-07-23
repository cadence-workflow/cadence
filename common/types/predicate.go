package types

type PredicateType int32

func (p PredicateType) Ptr() *PredicateType {
	return &p
}

const (
	PredicateTypeUniversal PredicateType = iota
	PredicateTypeEmpty
	PredicateTypeAnd
	PredicateTypeOr
	PredicateTypeNot
	PredicateTypeDomainID

	NumPredicateTypes
)

type UniversalPredicateAttributes struct{}

func (u *UniversalPredicateAttributes) Copy() *UniversalPredicateAttributes {
	if u == nil {
		return nil
	}
	return &UniversalPredicateAttributes{}
}

type EmptyPredicateAttributes struct{}

func (e *EmptyPredicateAttributes) Copy() *EmptyPredicateAttributes {
	if e == nil {
		return nil
	}
	return &EmptyPredicateAttributes{}
}

type AndPredicateAttributes struct {
	Predicates []*Predicate
}

func (a *AndPredicateAttributes) Copy() *AndPredicateAttributes {
	if a == nil {
		return nil
	}
	var predicates []*Predicate
	if a.Predicates != nil {
		predicates = make([]*Predicate, len(a.Predicates))
		for i, predicate := range a.Predicates {
			predicates[i] = predicate.Copy()
		}
	}
	return &AndPredicateAttributes{
		Predicates: predicates,
	}
}

type OrPredicateAttributes struct {
	Predicates []*Predicate
}

func (o *OrPredicateAttributes) Copy() *OrPredicateAttributes {
	if o == nil {
		return nil
	}
	var predicates []*Predicate
	if o.Predicates != nil {
		predicates = make([]*Predicate, len(o.Predicates))
		for i, predicate := range o.Predicates {
			predicates[i] = predicate.Copy()
		}
	}
	return &OrPredicateAttributes{
		Predicates: predicates,
	}
}

type NotPredicateAttributes struct {
	Predicate *Predicate
}

func (n *NotPredicateAttributes) Copy() *NotPredicateAttributes {
	if n == nil {
		return nil
	}
	return &NotPredicateAttributes{
		Predicate: n.Predicate.Copy(),
	}
}

type DomainIDPredicateAttributes struct {
	DomainIDs   []string
	IsExclusive *bool
}

func (d *DomainIDPredicateAttributes) Copy() *DomainIDPredicateAttributes {
	if d == nil {
		return nil
	}
	return &DomainIDPredicateAttributes{
		DomainIDs:   d.DomainIDs,
		IsExclusive: d.IsExclusive,
	}
}

type Predicate struct {
	PredicateType                PredicateType
	UniversalPredicateAttributes *UniversalPredicateAttributes
	EmptyPredicateAttributes     *EmptyPredicateAttributes
	AndPredicateAttributes       *AndPredicateAttributes
	OrPredicateAttributes        *OrPredicateAttributes
	NotPredicateAttributes       *NotPredicateAttributes
	DomainIDPredicateAttributes  *DomainIDPredicateAttributes
}

func (p *Predicate) Copy() *Predicate {
	if p == nil {
		return nil
	}
	return &Predicate{
		PredicateType:                p.PredicateType,
		UniversalPredicateAttributes: p.UniversalPredicateAttributes.Copy(),
		EmptyPredicateAttributes:     p.EmptyPredicateAttributes.Copy(),
		AndPredicateAttributes:       p.AndPredicateAttributes.Copy(),
		OrPredicateAttributes:        p.OrPredicateAttributes.Copy(),
		NotPredicateAttributes:       p.NotPredicateAttributes.Copy(),
		DomainIDPredicateAttributes:  p.DomainIDPredicateAttributes.Copy(),
	}
}

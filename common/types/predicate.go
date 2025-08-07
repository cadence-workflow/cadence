package types

type PredicateType int32

func (p PredicateType) Ptr() *PredicateType {
	return &p
}

const (
	PredicateTypeUniversal PredicateType = iota
	PredicateTypeEmpty
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

func (d *DomainIDPredicateAttributes) GetIsExclusive() bool {
	if d.IsExclusive == nil {
		return false
	}
	return *d.IsExclusive
}

type Predicate struct {
	PredicateType                PredicateType
	UniversalPredicateAttributes *UniversalPredicateAttributes
	EmptyPredicateAttributes     *EmptyPredicateAttributes
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
		DomainIDPredicateAttributes:  p.DomainIDPredicateAttributes.Copy(),
	}
}

func (p *Predicate) GetDomainIDPredicateAttributes() *DomainIDPredicateAttributes {
	if p == nil {
		return nil
	}
	if p.PredicateType != PredicateTypeDomainID {
		return nil
	}
	return p.DomainIDPredicateAttributes
}

package queuev2

import "fmt"

func Not(predicate Predicate) Predicate {
	switch p := predicate.(type) {
	case *universalPredicate:
		return &emptyPredicate{}
	case *emptyPredicate:
		return &universalPredicate{}
	case *domainIDPredicate:
		isExclusive := !p.isExclusive
		if isExclusive && len(p.domainIDs) == 0 {
			return &universalPredicate{}
		}
		return &domainIDPredicate{
			domainIDs:   p.domainIDs,
			isExclusive: isExclusive,
		}
	default:
		panic(fmt.Sprintf("unknown predicate type: %T", p))
	}
}

func And(p1, p2 Predicate) Predicate {
	switch p1 := p1.(type) {
	case *universalPredicate:
		return p2
	case *emptyPredicate:
		return p1
	case *domainIDPredicate:
		switch p2 := p2.(type) {
		case *universalPredicate:
			return p1
		case *emptyPredicate:
			return p2
		case *domainIDPredicate:
			if p1.isExclusive {
				if p2.isExclusive {
					domainIDs := unionStringSet(p1.domainIDs, p2.domainIDs)
					if len(domainIDs) == 0 {
						return &emptyPredicate{}
					}
					return &domainIDPredicate{
						domainIDs:   domainIDs,
						isExclusive: true,
					}
				}
				domainIDs := map[string]struct{}{}
				for domainID := range p2.domainIDs {
					if _, ok := p1.domainIDs[domainID]; !ok {
						domainIDs[domainID] = struct{}{}
					}
				}
				if len(domainIDs) == 0 {
					return &emptyPredicate{}
				}
				return &domainIDPredicate{
					domainIDs:   domainIDs,
					isExclusive: false,
				}
			}
			if p2.isExclusive {
				domainIDs := map[string]struct{}{}
				for domainID := range p1.domainIDs {
					if _, ok := p2.domainIDs[domainID]; !ok {
						domainIDs[domainID] = struct{}{}
					}
				}
				if len(domainIDs) == 0 {
					return &emptyPredicate{}
				}
				return &domainIDPredicate{
					domainIDs:   domainIDs,
					isExclusive: false,
				}
			}
			domainIDs := intersectStringSet(p1.domainIDs, p2.domainIDs)
			if len(domainIDs) == 0 {
				return &emptyPredicate{}
			}
			return &domainIDPredicate{
				domainIDs:   domainIDs,
				isExclusive: false,
			}

		default:
			panic(fmt.Sprintf("unknown predicate type: %T", p2))
		}
	default:
		panic(fmt.Sprintf("unknown predicate type: %T", p1))
	}
}

func Or(p1, p2 Predicate) Predicate {
	switch p1 := p1.(type) {
	case *universalPredicate:
		return p1
	case *emptyPredicate:
		return p2
	case *domainIDPredicate:
		switch p2 := p2.(type) {
		case *universalPredicate:
			return p2
		case *emptyPredicate:
			return p1
		case *domainIDPredicate:
			if p1.isExclusive {
				if p2.isExclusive {
					domainIDs := intersectStringSet(p1.domainIDs, p2.domainIDs)
					if len(domainIDs) == 0 {
						return &universalPredicate{}
					}
					return &domainIDPredicate{
						domainIDs:   domainIDs,
						isExclusive: true,
					}
				}
				domainIDs := diffStringSet(p1.domainIDs, p2.domainIDs)
				if len(domainIDs) == 0 {
					return &universalPredicate{}
				}
				return &domainIDPredicate{
					domainIDs:   domainIDs,
					isExclusive: true,
				}
			}
			if p2.isExclusive {
				domainIDs := diffStringSet(p2.domainIDs, p1.domainIDs)
				if len(domainIDs) == 0 {
					return &universalPredicate{}
				}
				return &domainIDPredicate{
					domainIDs:   domainIDs,
					isExclusive: true,
				}
			}
			domainIDs := unionStringSet(p1.domainIDs, p2.domainIDs)
			if len(domainIDs) == 0 {
				return &emptyPredicate{}
			}
			return &domainIDPredicate{
				domainIDs:   domainIDs,
				isExclusive: false,
			}
		default:
			panic(fmt.Sprintf("unknown predicate type: %T", p2))
		}
	default:
		panic(fmt.Sprintf("unknown predicate type: %T", p1))
	}
}

func unionStringSet(set1, set2 map[string]struct{}) map[string]struct{} {
	result := make(map[string]struct{})
	for domainID := range set1 {
		result[domainID] = struct{}{}
	}
	for domainID := range set2 {
		result[domainID] = struct{}{}
	}
	return result
}

func intersectStringSet(set1, set2 map[string]struct{}) map[string]struct{} {
	result := make(map[string]struct{})
	for domainID := range set1 {
		if _, ok := set2[domainID]; ok {
			result[domainID] = struct{}{}
		}
	}
	return result
}

func diffStringSet(set1, set2 map[string]struct{}) map[string]struct{} {
	result := make(map[string]struct{})
	for domainID := range set1 {
		if _, ok := set2[domainID]; !ok {
			result[domainID] = struct{}{}
		}
	}
	return result
}

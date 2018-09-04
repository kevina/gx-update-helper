package main

type DepSet map[Hash]struct{}

func (s DepSet) Has(hash Hash) bool {
	_, ok := s[hash]
	return ok
}

func (s DepSet) Len() int {
	return len(s)
}

func (s DepSet) Elms() []Hash {
	elms := make([]Hash, 0, len(s))
	for el, _ := range s {
		elms = append(elms, el)
	}
	return elms
}

func (s DepSet) Add(toAdd ...Hash) int {
	origLen := len(s)
	for _, hash := range toAdd {
		s[hash] = struct{}{}
	}
	return len(s) - origLen
}

func (s DepSet) Del(toDel ...Hash) int {
	origLen := len(s)
	for _, hash := range toDel {
		delete(s, hash)
	}
	return origLen - len(s)
}

func (s DepSet) Clone() DepSet {
	new := DepSet{}
	for el, _ := range s {
		new.Add(el)
	}
	return new
}

type NameSet map[string]struct{}

func (s NameSet) Has(hash string) bool {
	_, ok := s[hash]
	return ok
}

func (s NameSet) Add(toAdd ...string) int {
	origLen := len(s)
	for _, hash := range toAdd {
		s[hash] = struct{}{}
	}
	return len(s) - origLen
}

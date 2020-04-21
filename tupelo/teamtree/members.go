package teamtree

type MemberIface interface {
	Did() string
	Name() string
}

type Members []MemberIface

func (m Members) Dids() []string {
	dids := make([]string, len(m))
	for i, v := range m {
		dids[i] = v.Did()
	}
	return dids
}

func (m Members) Names() []string {
	names := make([]string, len(m))
	for i, v := range m {
		names[i] = v.Name()
	}
	return names
}

func (m Members) Map() map[string]string {
	ret := make(map[string]string)
	for _, v := range m {
		ret[v.Name()] = v.Did()
	}
	return ret
}

func (m Members) IsMember(did string) bool {
	for _, v := range m {
		if v.Did() == did {
			return true
		}
	}
	return false
}

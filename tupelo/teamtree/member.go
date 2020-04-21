package teamtree

type Member struct {
	MemberIface
	did  string
	name string
}

func NewMember(did string, name string) *Member {
	return &Member{
		did:  did,
		name: name,
	}
}

func (m *Member) Did() string {
	return m.did
}

func (m *Member) Name() string {
	return m.name
}

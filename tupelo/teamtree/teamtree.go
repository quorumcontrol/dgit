package teamtree

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"strings"

	logging "github.com/ipfs/go-log"
	"github.com/quorumcontrol/chaintree/chaintree"
	"github.com/quorumcontrol/dgit/tupelo/tree"
	"github.com/quorumcontrol/messages/v2/build/go/transactions"
	tupelo "github.com/quorumcontrol/tupelo/sdk/gossip/client"
)

var log = logging.Logger("decentragit.teamtree")

var ErrNotFound = tree.ErrNotFound

var membersPath = []string{"dgit", "members"}

type Options struct {
	Name    string
	Tupelo  *tupelo.Client
	Members Members
}

type TeamTree struct {
	*tree.Tree
}

func Find(ctx context.Context, client *tupelo.Client, did string) (*TeamTree, error) {
	log.Debugf("looking for team %s", did)
	t, err := tree.Find(ctx, client, did)
	if err != nil {
		return nil, err
	}
	return &TeamTree{t}, nil
}

func Create(ctx context.Context, opts *Options) (*TeamTree, error) {
	membersTxn, err := chaintree.NewSetDataTransaction(strings.Join(membersPath, "/"), opts.Members.Map())
	if err != nil {
		return nil, err
	}

	t, err := tree.Create(ctx, &tree.Options{
		Name:           opts.Name,
		Tupelo:         opts.Tupelo,
		Owners:         opts.Members.Dids(),
		AdditionalTxns: []*transactions.Transaction{membersTxn},
	})
	if err != nil {
		return nil, err
	}
	log.Infof("created %s team chaintree with did: %s", t.Name(), t.Did())

	return &TeamTree{t}, nil
}

func (t *TeamTree) AddMembers(ctx context.Context, key *ecdsa.PrivateKey, members Members) error {
	currentMembers, err := t.ListMembers(ctx)
	if err != nil {
		return err
	}
	return t.SetMembers(ctx, key, append(currentMembers, members...))
}

func (t *TeamTree) ListMembers(ctx context.Context) (Members, error) {
	path := append([]string{"tree", "data"}, membersPath...)
	valUncast, _, err := t.Resolve(ctx, path)
	if err != nil {
		return nil, err
	}
	if valUncast == nil {
		return Members{}, nil
	}

	valMapUncast, ok := valUncast.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("path %v is %T, expected map", path, valUncast)
	}

	members := make(Members, len(valMapUncast))
	i := 0

	for name, v := range valMapUncast {
		did, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("key %s at path %v is %T, expected string", name, path, v)
		}
		members[i] = NewMember(did, name)
		i++
	}
	return members, nil
}

func (t *TeamTree) RemoveMembers(ctx context.Context, key *ecdsa.PrivateKey, membersToRemove Members) error {
	currentMembers, err := t.ListMembers(ctx)
	if err != nil {
		return err
	}

	members := Members{}
	for _, member := range currentMembers {
		if !membersToRemove.IsMember(member.Did()) {
			members = append(members, member)
		}
	}

	return t.SetMembers(ctx, key, members)
}

func (t *TeamTree) SetMembers(ctx context.Context, key *ecdsa.PrivateKey, members Members) error {
	ownershipTxn, err := chaintree.NewSetOwnershipTransaction(members.Dids())
	if err != nil {
		return err
	}

	membersTxn, err := chaintree.NewSetDataTransaction(strings.Join(membersPath, "/"), members.Map())
	if err != nil {
		return err
	}

	_, err = t.Tupelo().PlayTransactions(ctx, t.ChainTree(), key, []*transactions.Transaction{ownershipTxn, membersTxn})
	return err
}

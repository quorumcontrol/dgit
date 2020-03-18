package chaintree

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"

	"gopkg.in/src-d/go-git.v4/storage/memory"

	"github.com/quorumcontrol/dgit/storage"

	format "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log"
	"github.com/quorumcontrol/chaintree/chaintree"
	"github.com/quorumcontrol/messages/v2/build/go/transactions"
	"go.uber.org/zap"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/objfile"
	"github.com/go-git/go-git/v5/plumbing/storer"
)

var log = logging.Logger("dgit.storage.chaintree")

type ObjectStorage struct {
	*storage.ChaintreeObjectStorage
	log            *zap.SugaredLogger
	packfileWriter io.WriteCloser
}

var _ storage.ChaintreeObjectStorer = (*ObjectStorage)(nil)
var _ storer.PackfileWriter = (*ObjectStorage)(nil)
var _ storer.Transactioner = (*ObjectStorage)(nil)

func NewObjectStorage(config *storage.Config) storer.EncodedObjectStorer {
	did := config.ChainTree.MustId()
	return &ObjectStorage{
		&storage.ChaintreeObjectStorage{config},
		log.Named(did[len(did)-6:]),
		nil,
	}
}

type ObjectTransaction struct {
	temporal storer.EncodedObjectStorer
	storage  storer.EncodedObjectStorer
}

var _ storer.Transaction = (*ObjectTransaction)(nil)

func (s *ObjectStorage) Begin() storer.Transaction {
	return &ObjectTransaction{
		temporal: memory.NewStorage(),
		storage:  s,
	}
}

func (ot *ObjectTransaction) SetEncodedObject(o plumbing.EncodedObject) (plumbing.Hash, error) {
	return ot.temporal.SetEncodedObject(o)
}

func (ot *ObjectTransaction) EncodedObject(t plumbing.ObjectType, h plumbing.Hash) (plumbing.EncodedObject, error) {
	return ot.temporal.EncodedObject(t, h)
}

func (ot *ObjectTransaction) Commit() error {
	iter, err := ot.temporal.IterEncodedObjects(plumbing.AnyObject)
	if err != nil {
		return err
	}

	tupeloTxns := make([]*transactions.Transaction, 0)

	ctStorage, ok := ot.storage.(*ObjectStorage)
	if !ok {
		return fmt.Errorf("could not cast storage to chaintree.ObjectStorage; was %T", ot.storage)
	}

	err = iter.ForEach(func(o plumbing.EncodedObject) error {
		tupeloTxn, err := ctStorage.SetEncodedObjectTxn(o)
		if err != nil {
			return err
		}

		tupeloTxns = append(tupeloTxns, tupeloTxn)

		return nil
	})

	if len(tupeloTxns) > 0 {
		_, err = ctStorage.Tupelo.PlayTransactions(ctStorage.Ctx, ctStorage.ChainTree, ctStorage.PrivateKey, tupeloTxns)
		if err != nil {
			return err
		}
	}

	return nil
}

func (ot *ObjectTransaction) Rollback() error {
	ot.temporal = nil
	return nil
}

func (s *ObjectStorage) PackfileWriter() (io.WriteCloser, error) {
	return storage.NewPackWriter(s), nil
}

func (s *ObjectStorage) SetEncodedObjectTxn(o plumbing.EncodedObject) (*transactions.Transaction, error) {
	s.log.Debugf("saving %s with type %s", o.Hash().String(), o.Type().String())

	if s.PrivateKey == nil {
		return nil, fmt.Errorf("Must specify treeKey during NewObjectStorage init")
	}

	if o.Type() == plumbing.OFSDeltaObject || o.Type() == plumbing.REFDeltaObject {
		return nil, plumbing.ErrInvalidType
	}

	buf := bytes.NewBuffer(nil)

	writer := objfile.NewWriter(buf)
	defer writer.Close()

	reader, err := o.Reader()
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	if err := writer.WriteHeader(o.Type(), o.Size()); err != nil {
		return nil, err
	}

	if _, err = io.Copy(writer, reader); err != nil {
		return nil, err
	}
	writer.Close()

	objectBytes, err := ioutil.ReadAll(buf)
	if err != nil {
		return nil, err
	}

	// TODO: save each git object as cid
	//   currently objects/sha1[0:2]/ is a map with { sha1[2:] => cbor bytes }
	//   should be objects/sha1[0:2]/ is a map with { sha1[2:] => cid }
	transaction, err := chaintree.NewSetDataTransaction(storage.ObjectWritePath(o.Hash()), objectBytes)
	if err != nil {
		return nil, err
	}

	return transaction, nil
}

func (s *ObjectStorage) SetEncodedObject(o plumbing.EncodedObject) (plumbing.Hash, error) {
	transaction, err := s.SetEncodedObjectTxn(o)
	if err != nil {
		return plumbing.ZeroHash, err
	}

	_, err = s.Tupelo.PlayTransactions(s.Ctx, s.ChainTree, s.PrivateKey, []*transactions.Transaction{transaction})
	if err != nil {
		return plumbing.ZeroHash, err
	}

	return o.Hash(), nil
}

func (s *ObjectStorage) HasEncodedObject(h plumbing.Hash) (err error) {
	if _, err := s.EncodedObject(plumbing.AnyObject, h); err != nil {
		return err
	}
	return nil
}

func (s *ObjectStorage) EncodedObjectSize(h plumbing.Hash) (size int64, err error) {
	o, err := s.EncodedObject(plumbing.AnyObject, h)
	if err != nil {
		return 0, err
	}
	return o.Size(), nil
}

func (s *ObjectStorage) EncodedObject(t plumbing.ObjectType, h plumbing.Hash) (plumbing.EncodedObject, error) {
	s.log.Debugf("fetching %s with type %s", h.String(), t.String())

	valUncast, _, err := s.ChainTree.ChainTree.Dag.Resolve(s.Ctx, storage.ObjectReadPath(h))
	if err == format.ErrNotFound {
		s.log.Debugf("%s not found", h.String())
		return nil, plumbing.ErrObjectNotFound
	}
	if err != nil {
		s.log.Errorf("chaintree resolve error for %s: %v", h.String(), err)
		return nil, err
	}
	if valUncast == nil {
		s.log.Debugf("%s not found", h.String())
		return nil, plumbing.ErrObjectNotFound
	}

	o := &plumbing.MemoryObject{}

	buf := bytes.NewBuffer(valUncast.([]byte))
	reader, err := objfile.NewReader(buf)
	if err != nil {
		s.log.Errorf("new reader error for %s: %v", h.String(), err)
		return nil, err
	}
	defer reader.Close()

	objType, size, err := reader.Header()
	if err != nil {
		s.log.Errorf("error decoding header for %s: %v", h.String(), err)
		return nil, err
	}

	o.SetType(objType)
	o.SetSize(size)

	if plumbing.AnyObject != t && o.Type() != t {
		s.log.Debugf("%s not found, mismatched types, expected %s, got %s", h.String(), t.String(), o.Type().String())
		return nil, plumbing.ErrObjectNotFound
	}

	if _, err = io.Copy(o, reader); err != nil {
		s.log.Errorf("error filling object %s: %v", h.String(), err)
		return nil, err
	}

	return o, nil
}

func (s *ObjectStorage) IterEncodedObjects(t plumbing.ObjectType) (storer.EncodedObjectIter, error) {
	return storage.NewEncodedObjectIter(s, t), nil
}

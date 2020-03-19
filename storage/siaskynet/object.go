package siaskynet

import (
	"bytes"
	"io"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/format/objfile"
	format "github.com/ipfs/go-ipld-format"
	"github.com/quorumcontrol/messages/v2/build/go/transactions"

	"github.com/quorumcontrol/dgit/storage"

	"github.com/NebulousLabs/go-skynet"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/storer"
	logging "github.com/ipfs/go-log"
	"github.com/quorumcontrol/chaintree/chaintree"
	"go.uber.org/zap"
)

var log = logging.Logger("dgit.storage.siaskynet")

type ObjectStorage struct {
	*storage.ChaintreeObjectStorage
	log *zap.SugaredLogger
}

var _ storer.EncodedObjectStorer = (*ObjectStorage)(nil)
var _ storer.PackfileWriter = (*ObjectStorage)(nil)
var _ storer.Transactioner = (*ObjectStorage)(nil)

func NewObjectStorage(config *storage.Config) storer.EncodedObjectStorer {
	did := config.ChainTree.MustId()
	return &ObjectStorage{
		&storage.ChaintreeObjectStorage{config},
		log.Named(did[len(did)-6:]),
	}
}

type TemporalStorage struct {
	log      *zap.SugaredLogger
	skylinks map[plumbing.Hash]string
}

type ChaintreeLinkStorage struct {
	log *zap.SugaredLogger
	*storage.Config
}

func NewTemporalStorage() *TemporalStorage {
	return &TemporalStorage{
		log:      log.Named("skynet-temporal"),
		skylinks: make(map[plumbing.Hash]string),
	}
}

func NewChaintreeLinkStorage(config *storage.Config) *ChaintreeLinkStorage {
	did := config.ChainTree.MustId()
	return &ChaintreeLinkStorage{
		log.Named(did[len(did)-6:]),
		config,
	}
}

func uploadObjectToSkynet(o plumbing.EncodedObject) (string, error) {
	buf := bytes.NewBuffer(nil)

	writer := objfile.NewWriter(buf)
	defer writer.Close()

	reader, err := o.Reader()
	if err != nil {
		return "", err
	}

	if err = writer.WriteHeader(o.Type(), o.Size()); err != nil {
		return "", err
	}

	if _, err = io.Copy(writer, reader); err != nil {
		return "", err
	}

	uploadData := make(skynet.UploadData)
	uploadData[o.Hash().String()] = buf

	link, err := skynet.Upload(uploadData, skynet.DefaultUploadOptions)

	return link, nil
}

func (ts *TemporalStorage) SetEncodedObject(o plumbing.EncodedObject) (plumbing.Hash, error) {
	ts.log.Debugf("uploading %s to Skynet", o.Hash())

	link, err := uploadObjectToSkynet(o)
	if err != nil {
		return plumbing.ZeroHash, err
	}

	objHash := o.Hash()

	ts.skylinks[objHash] = link

	return objHash, nil
}

func downloadObjectFromSkynet(link string) (plumbing.EncodedObject, error) {
	objData, err := skynet.Download(link, skynet.DefaultDownloadOptions)
	if err != nil {
		return nil, err
	}

	o := &plumbing.MemoryObject{}

	reader, err := objfile.NewReader(objData)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	objType, size, err := reader.Header()
	if err != nil {
		return nil, err
	}

	o.SetType(objType)
	o.SetSize(size)

	if _, err = io.Copy(o, reader); err != nil {
		return nil, err
	}

	return o, nil
}

func (ts *TemporalStorage) EncodedObject(t plumbing.ObjectType, h plumbing.Hash) (plumbing.EncodedObject, error) {
	link, ok := ts.skylinks[h]
	if !ok {
		return nil, plumbing.ErrObjectNotFound
	}

	o, err := downloadObjectFromSkynet(link)
	if err != nil {
		ts.log.Errorf("could not download object %s from Skynet: %w", h, err)
		return nil, err
	}

	if plumbing.AnyObject != t && o.Type() != t {
		ts.log.Debugf("%s not found, mismatched types, expected %s, got %s", h, t, o.Type())
		return nil, plumbing.ErrObjectNotFound
	}

	return o, nil
}

type ObjectTransaction struct {
	temporal *TemporalStorage
	storage  *ChaintreeLinkStorage
	log      *zap.SugaredLogger
}

var _ storer.Transaction = (*ObjectTransaction)(nil)

func (s *ObjectStorage) Begin() storer.Transaction {
	ts := NewTemporalStorage()
	ls := NewChaintreeLinkStorage(s.Config)
	return &ObjectTransaction{
		// NB: Currently TemporalStorage uploads objects to
		// skynet as they are added to the txn. This makes sense while it's
		// free, but perhaps less so once it isn't. It still might make sense
		// perf-wise, but you'd want to clean up on Rollback / error to stop
		// paying for those uploads.
		temporal: ts,
		storage:  ls,
		log:      s.log.Named("object-transaction"),
	}
}

func (ot *ObjectTransaction) SetEncodedObject(o plumbing.EncodedObject) (plumbing.Hash, error) {
	ot.log.Debugf("added object %s to transaction", o.Hash())
	return ot.temporal.SetEncodedObject(o)
}

func (ot *ObjectTransaction) EncodedObject(t plumbing.ObjectType, h plumbing.Hash) (plumbing.EncodedObject, error) {
	ot.log.Debugf("retrieving object %s - %s from transaction", t, h)
	return ot.temporal.EncodedObject(t, h)
}

func (ot *ObjectTransaction) Commit() error {
	ot.log.Debugf("committing transaction")

	var tupeloTxns []*transactions.Transaction

	for h, link := range ot.temporal.skylinks {
		txn, err := setLinkTxn(h, strings.Replace(link, "sia://", "did:sia:", 1))
		if err != nil {
			return err
		}

		tupeloTxns = append(tupeloTxns, txn)
	}

	if len(ot.temporal.skylinks) > 0 {
		ot.log.Debugf("saving %d Skylinks in transaction to repo chaintree", len(ot.temporal.skylinks))
		_, err := ot.storage.Tupelo.PlayTransactions(ot.storage.Ctx, ot.storage.ChainTree, ot.storage.PrivateKey, tupeloTxns)
		if err != nil {
			return err
		}
	}

	return nil
}

func setLinkTxn(h plumbing.Hash, link string) (*transactions.Transaction, error) {
	writePath := storage.ObjectWritePath(h)

	txn, err := chaintree.NewSetDataTransaction(writePath, link)
	if err != nil {
		return nil, err
	}

	return txn, nil
}

func (ot *ObjectTransaction) Rollback() error {
	ot.log.Debugf("rolling back transaction")
	ot.temporal = nil
	return nil
}

func (s *ObjectStorage) PackfileWriter() (io.WriteCloser, error) {
	s.log.Debug("packfile writer requested")
	return storage.NewPackWriter(s), nil
}

func (s *ObjectStorage) SetEncodedObject(o plumbing.EncodedObject) (plumbing.Hash, error) {
	s.log.Debugf("saving %s with type %s", o.Hash().String(), o.Type().String())

	if o.Type() == plumbing.OFSDeltaObject || o.Type() == plumbing.REFDeltaObject {
		return plumbing.ZeroHash, plumbing.ErrInvalidType
	}

	s.log.Debugf("uploading %s to Skynet", o.Hash().String())
	link, err := uploadObjectToSkynet(o)
	if err != nil {
		return plumbing.ZeroHash, err
	}

	skylink := strings.TrimPrefix(link, "sia://")
	objDid := strings.Join([]string{"did", "sia", skylink}, ":")

	tx, err := setLinkTxn(o.Hash(), objDid)
	if err != nil {
		return plumbing.ZeroHash, err
	}

	_, err = s.Tupelo.PlayTransactions(s.Ctx, s.ChainTree, s.PrivateKey, []*transactions.Transaction{tx})
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
		s.log.Debugf("%s not found", h)
		return nil, plumbing.ErrObjectNotFound
	}
	if err != nil {
		s.log.Errorf("chaintree resolve error for %s: %w", h, err)
		return nil, err
	}
	if valUncast == nil {
		s.log.Debugf("%s not found", h)
		return nil, plumbing.ErrObjectNotFound
	}

	// TODO: Read these in higher-level code and delegate decoding to whichever
	//  object storage system is specified in the did:storer: prefix
	objDid, ok := valUncast.(string)
	if !ok {
		s.log.Errorf("object DID should be a string; was a %T instead", valUncast)
		return nil, plumbing.ErrObjectNotFound
	}
	if !strings.HasPrefix(objDid, "did:sia:") {
		s.log.Errorf("object DID %s should start with did:sia:", objDid)
		return nil, plumbing.ErrObjectNotFound
	}

	link := strings.Replace(objDid, "did:sia:", "sia://", 1)

	s.log.Debugf("downloading %s from Skynet at %s", h, link)
	o, err := downloadObjectFromSkynet(link)
	if err != nil {
		s.log.Errorf("could not download object %s from Skynet at %s: %w", o.Hash(), link, err)
		return nil, err
	}

	if plumbing.AnyObject != t && o.Type() != t {
		s.log.Debugf("%s not found, mismatched types, expected %s, got %s", h, t, o.Type())
		return nil, plumbing.ErrObjectNotFound
	}

	return o, nil
}

func (s *ObjectStorage) IterEncodedObjects(t plumbing.ObjectType) (storer.EncodedObjectIter, error) {
	return storage.NewEncodedObjectIter(s, t), nil
}

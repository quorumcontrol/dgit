package siaskynet

import (
	"bytes"
	"io"
	"strings"

	format "github.com/ipfs/go-ipld-format"
	"github.com/quorumcontrol/messages/v2/build/go/transactions"
	"gopkg.in/src-d/go-git.v4/plumbing/format/objfile"

	"github.com/quorumcontrol/decentragit-remote/storage"

	"github.com/NebulousLabs/go-skynet"
	logging "github.com/ipfs/go-log"
	"github.com/quorumcontrol/chaintree/chaintree"
	"go.uber.org/zap"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/storer"
)

var log = logging.Logger("dgit.storage.siaskynet")

type ObjectStorage struct {
	*storage.ChaintreeObjectStorage
	log *zap.SugaredLogger
}

var _ storer.EncodedObjectStorer = (*ObjectStorage)(nil)

func NewObjectStorage(config *storage.Config) storer.EncodedObjectStorer {
	did := config.ChainTree.MustId()
	return &ObjectStorage{
		&storage.ChaintreeObjectStorage{config},
		log.Named(did[len(did)-6:]),
	}
}

func (s *ObjectStorage) SetEncodedObject(o plumbing.EncodedObject) (plumbing.Hash, error) {
	s.log.Debugf("saving %s with type %s", o.Hash().String(), o.Type().String())

	if o.Type() == plumbing.OFSDeltaObject || o.Type() == plumbing.REFDeltaObject {
		return plumbing.ZeroHash, plumbing.ErrInvalidType
	}

	buf := bytes.NewBuffer(nil)

	writer := objfile.NewWriter(buf)
	defer writer.Close()

	reader, err := o.Reader()
	if err != nil {
		return plumbing.ZeroHash, err
	}

	if err = writer.WriteHeader(o.Type(), o.Size()); err != nil {
		return plumbing.ZeroHash, err
	}

	if _, err = io.Copy(writer, reader); err != nil {
		return plumbing.ZeroHash, err
	}
	writer.Close()

	uploadData := make(skynet.UploadData)
	uploadData[o.Hash().String()] = buf

	s.log.Debugf("uploading %s to Skynet", o.Hash().String())
	link, err := skynet.Upload(uploadData, skynet.DefaultUploadOptions.UploadOptions)

	skylink := strings.TrimPrefix(link, "sia://")
	objDid := strings.Join([]string{"did", "sia", skylink}, ":")

	writePath := storage.ObjectWritePath(o.Hash())
	s.log.Debugf("saving Skylink %s to repo chaintree at %s", objDid, writePath)

	tx, err := chaintree.NewSetDataTransaction(writePath, objDid)
	if err != nil {
		return plumbing.ZeroHash, err
	}

	_, err = s.Tupelo.PlayTransactions(s.Ctx, s.ChainTree, s.PrivateKey, []*transactions.Transaction{tx})
	if err != nil {
		return plumbing.ZeroHash, err
	}

	return o.Hash(), nil
}

// TODO: DRY these up with chaintree/object.go

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
		s.log.Errorf("chaintree resolve error for %s: %w", h.String(), err)
		return nil, err
	}
	if valUncast == nil {
		s.log.Debugf("%s not found", h.String())
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

	skylink := strings.TrimPrefix(objDid, "did:sia:")

	s.log.Debugf("downloading %s from Skynet", h.String())
	data, err := skynet.Download(skylink, skynet.DefaultDownloadOptions)
	if err != nil {
		s.log.Errorf("could not download skylink %s from Skynet: %w", skylink, err)
		return nil, err
	}

	o := &plumbing.MemoryObject{}

	reader, err := objfile.NewReader(data)
	if err != nil {
		s.log.Errorf("new reader error for %s: %w", h.String(), err)
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

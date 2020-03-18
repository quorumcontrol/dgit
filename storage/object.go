package storage

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/packfile"
	"github.com/go-git/go-git/v5/plumbing/storer"
	logging "github.com/ipfs/go-log"
	"github.com/quorumcontrol/chaintree/chaintree"
	"go.uber.org/zap"
)

var log = logging.Logger("dgit.storage.object")

type ChaintreeObjectStorer interface {
	storer.EncodedObjectStorer
	Chaintree() *chaintree.ChainTree
}

type ChaintreeObjectStorage struct {
	*Config
}

var ObjectsBasePath = []string{"tree", "data", "objects"}

func ObjectReadPath(h plumbing.Hash) []string {
	prefix := h.String()[0:2]
	key := h.String()[2:]
	return append(ObjectsBasePath, prefix, key)
}

func ObjectWritePath(h plumbing.Hash) string {
	return strings.Join(ObjectReadPath(h)[2:], "/")
}

func (s *ChaintreeObjectStorage) Chaintree() *chaintree.ChainTree {
	return s.ChainTree.ChainTree
}

func (s *ChaintreeObjectStorage) NewEncodedObject() plumbing.EncodedObject {
	return &plumbing.MemoryObject{}
}

type PackWriter struct {
	bytes   *bytes.Buffer
	closed  bool
	storage ChaintreeObjectStorer
	log     *zap.SugaredLogger
}

func NewPackWriter(s ChaintreeObjectStorer) *PackWriter {
	return &PackWriter{
		bytes:   nil,
		closed:  false,
		storage: s,
		log:     log.Named("pack writer"),
	}
}

func (pw *PackWriter) Write(p []byte) (n int, err error) {
	pw.log.Debugf("writing %d bytes", len(p))
	if pw.closed {
		return 0, fmt.Errorf("attempt to write to closed ChaintreePackWriter")
	}

	if pw.bytes == nil {
		buf := make([]byte, len(p))
		pw.bytes = bytes.NewBuffer(buf)
	} else {
		pw.bytes.Grow(len(p))
	}

	var written int64
	written, err = io.Copy(pw.bytes, bytes.NewReader(p))
	n = int(written)
	return
}

func (pw *PackWriter) Close() error {
	pw.log.Debug("closing")
	pw.closed = true
	return pw.save()
}

func (pw *PackWriter) save() error {
	if !pw.closed {
		return fmt.Errorf("ChaintreePackWriter should be closed before saving")
	}

	scanner := packfile.NewScanner(pw.bytes)

	po := &PackfileObserver{
		storage: pw.storage,
		log:     pw.log.Named("packfile observer"),
	}

	parser, err := packfile.NewParser(scanner, po)
	if err != nil {
		return err
	}

	pw.log.Debug("parsing packfile")
	_, err = parser.Parse()
	return err
}

type PackfileObserver struct {
	currentObject *plumbing.MemoryObject
	currentTxn    storer.Transaction
	storage       ChaintreeObjectStorer
	log           *zap.SugaredLogger
}

func (po *PackfileObserver) OnHeader(_ uint32) error {
	po.log.Debug("packfile header")
	return nil
}

func (po *PackfileObserver) OnInflatedObjectHeader(t plumbing.ObjectType, objSize, _ int64) error {
	po.log.Debugf("object header: %s", t)

	if po.currentObject != nil {
		return fmt.Errorf("got new object header before content was written")
	}

	po.currentObject = &plumbing.MemoryObject{}
	po.currentObject.SetType(t)
	po.currentObject.SetSize(objSize)

	return nil
}

func (po *PackfileObserver) OnInflatedObjectContent(h plumbing.Hash, _ int64, _ uint32, content []byte) error {
	po.log.Debugf("object content: %s", h)

	if po.currentObject == nil {
		return fmt.Errorf("got object content before header")
	}

	_, err := po.currentObject.Write(content)
	if err != nil {
		return err
	}

	txnStore, ok := po.storage.(storer.Transactioner)
	if !ok {
		return fmt.Errorf("storage does not support transactions")
	}

	if po.currentTxn == nil {
		po.log.Debug("beginning transaction")
		po.currentTxn = txnStore.Begin()
	}

	po.log.Debugf("adding current object to transaction: %+v", po.currentObject)
	_, err = po.currentTxn.SetEncodedObject(po.currentObject)
	if err != nil {
		return err
	}

	po.currentObject = nil

	return nil
}

func (po *PackfileObserver) OnFooter(_ plumbing.Hash) error {
	po.log.Debug("packfile footer; committing current transaction")
	err := po.currentTxn.Commit()

	po.currentTxn = nil

	return err
}

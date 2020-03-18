package storage

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/packfile"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/quorumcontrol/chaintree/chaintree"
)

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
}

func NewPackWriter(s ChaintreeObjectStorer) *PackWriter {
	return &PackWriter{
		bytes:   nil,
		closed:  false,
		storage: s,
	}
}

func (pw *PackWriter) Write(p []byte) (n int, err error) {
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
	pw.closed = true
	return pw.save()
}

func (pw *PackWriter) save() error {
	if !pw.closed {
		return fmt.Errorf("ChaintreePackWriter should be closed before saving")
	}

	scanner := packfile.NewScanner(pw.bytes)

	cpo := &PackfileObserver{
		storage: pw.storage,
	}

	parser, err := packfile.NewParser(scanner, cpo)
	if err != nil {
		return err
	}

	_, err = parser.Parse()
	return err
}

type PackfileObserver struct {
	currentObject *plumbing.MemoryObject
	currentTxn    storer.Transaction
	storage       ChaintreeObjectStorer
}

func (po *PackfileObserver) OnHeader(_ uint32) error {
	return nil
}

func (po *PackfileObserver) OnInflatedObjectHeader(t plumbing.ObjectType, objSize, _ int64) error {
	if po.currentObject != nil {
		return fmt.Errorf("got new object header before content was written")
	}

	po.currentObject = &plumbing.MemoryObject{}
	po.currentObject.SetType(t)
	po.currentObject.SetSize(objSize)

	return nil
}

func (po *PackfileObserver) OnInflatedObjectContent(_ plumbing.Hash, _ int64, _ uint32, content []byte) error {
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
		po.currentTxn = txnStore.Begin()
	}

	_, err = po.currentTxn.SetEncodedObject(po.currentObject)
	if err != nil {
		return err
	}

	po.currentObject = nil

	return nil
}

func (po *PackfileObserver) OnFooter(_ plumbing.Hash) error {
	err := po.currentTxn.Commit()

	po.currentTxn = nil

	return err
}

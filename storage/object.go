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

func (cpw *PackWriter) Write(p []byte) (n int, err error) {
	if cpw.closed {
		return 0, fmt.Errorf("attempt to write to closed ChaintreePackWriter")
	}

	if cpw.bytes == nil {
		buf := make([]byte, len(p))
		cpw.bytes = bytes.NewBuffer(buf)
	} else {
		cpw.bytes.Grow(len(p))
	}

	var written int64
	written, err = io.Copy(cpw.bytes, bytes.NewReader(p))
	n = int(written)
	return
}

func (cpw *PackWriter) Close() error {
	cpw.closed = true
	return cpw.save()
}

func (cpw *PackWriter) save() error {
	if !cpw.closed {
		return fmt.Errorf("ChaintreePackWriter should be closed before saving")
	}

	scanner := packfile.NewScanner(cpw.bytes)

	cpo := &PackfileObserver{
		storage: cpw.storage,
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

func (cpo *PackfileObserver) OnHeader(_ uint32) error {
	return nil
}

func (cpo *PackfileObserver) OnInflatedObjectHeader(t plumbing.ObjectType, objSize, _ int64) error {
	if cpo.currentObject != nil {
		return fmt.Errorf("got new object header before content was written")
	}

	cpo.currentObject = &plumbing.MemoryObject{}
	cpo.currentObject.SetType(t)
	cpo.currentObject.SetSize(objSize)

	return nil
}

func (cpo *PackfileObserver) OnInflatedObjectContent(_ plumbing.Hash, _ int64, _ uint32, content []byte) error {
	if cpo.currentObject == nil {
		return fmt.Errorf("got object content before header")
	}

	_, err := cpo.currentObject.Write(content)
	if err != nil {
		return err
	}

	txnStore, ok := cpo.storage.(storer.Transactioner)
	if !ok {
		return fmt.Errorf("storage does not support transactions")
	}

	if cpo.currentTxn == nil {
		cpo.currentTxn = txnStore.Begin()
	}
	_, err = cpo.currentTxn.SetEncodedObject(cpo.currentObject)
	if err != nil {
		return err
	}

	cpo.currentObject = nil

	return nil
}

func (cpo *PackfileObserver) OnFooter(_ plumbing.Hash) error {
	err := cpo.currentTxn.Commit()

	cpo.currentTxn = nil

	return err
}

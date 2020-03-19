package siaskynet

import (
	"bytes"
	"io"

	"github.com/NebulousLabs/go-skynet"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/objfile"
	"go.uber.org/zap"
)

type uploadJob struct {
	o      plumbing.EncodedObject
	result chan string
	err    chan error
}

type downloadJob struct {
	link   string
	result chan plumbing.EncodedObject
	err    chan error
}

type Skynet struct {
	uploaderCount      int
	downloaderCount    int
	uploadersStarted   bool
	downloadersStarted bool
	uploadJobs         chan *uploadJob
	downloadJobs       chan *downloadJob

	log *zap.SugaredLogger
}

func InitSkynet(uploaderCount, downloaderCount int) *Skynet {
	return &Skynet{
		uploaderCount:   uploaderCount,
		downloaderCount: downloaderCount,
		uploadJobs:      make(chan *uploadJob),
		downloadJobs:    make(chan *downloadJob),
		log:             log.Named("net"),
	}
}

func (s *Skynet) uploadObject(o plumbing.EncodedObject) (string, error) {
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

func (s *Skynet) startUploader() {
	for j := range s.uploadJobs {
		s.log.Debugf("uploading %s to Skynet", j.o.Hash())
		link, err := s.uploadObject(j.o)
		if err != nil {
			j.err <- err
			continue
		}
		j.result <- link
	}
}

func (s *Skynet) startUploaders() {
	s.log.Debugf("starting %d uploaders", s.uploaderCount)

	for i := 0; i < s.uploaderCount; i++ {
		go s.startUploader()
	}
}

func (s *Skynet) UploadObject(o plumbing.EncodedObject) (chan string, chan error) {
	if !s.uploadersStarted {
		s.startUploaders()
		s.uploadersStarted = true
	}

	result := make(chan string)
	err := make(chan error)

	s.uploadJobs <- &uploadJob{
		o:      o,
		result: result,
		err:    err,
	}

	return result, err
}

package repo

import (
	"fmt"
	"os"
	"path"

	"github.com/quorumcontrol/decentragit-remote/constants"
)

type Repo struct {
	remoteName string
	url        string
	protocol   string
}

func New(remoteName string, url string) (*Repo, error) {
	r := &Repo{
		remoteName: remoteName,
		url:        url,
		protocol:   constants.Protocol,
	}

	return r, r.Initialize()
}

func (r *Repo) RemoteName() string {
	return r.remoteName
}

func (r *Repo) Url() string {
	return r.url
}

func (r *Repo) Dir() string {
	return path.Join(os.Getenv("GIT_DIR"), r.protocol, r.RemoteName())
}

func (r *Repo) HeadRefs() string {
	return fmt.Sprintf("refs/heads/*:refs/%s/%s/*", r.protocol, r.RemoteName())
}

func (r *Repo) BranchRefs() string {
	return fmt.Sprintf("refs/heads/branches/*:refs/%s/%s/branches/*", r.protocol, r.RemoteName())
}

func (r *Repo) TagRefs() string {
	return fmt.Sprintf("refs/heads/tags/*:refs/%s/%s/tags/*", r.protocol, r.RemoteName())
}

func (r *Repo) Capabilities() []string {
	return []string{
		"push",
		"fetch",
		// "import",
		// "export",
		fmt.Sprintf("refspec %s", r.HeadRefs()),
		fmt.Sprintf("refspec %s", r.BranchRefs()),
		fmt.Sprintf("refspec %s", r.TagRefs()),
		// fmt.Sprintf("*import-marks %s", gitMarks),
		// fmt.Sprintf("*export-marks %s", gitMarks),
	}
}

func (r *Repo) Initialize() error {
	if err := os.MkdirAll(r.Dir(), 0755); err != nil {
		return err
	}

	if err := r.touch("git.marks"); err != nil {
		return err
	}

	if err := r.touch(fmt.Sprintf("%s.marks", r.protocol)); err != nil {
		return err
	}

	return nil
}

func (r *Repo) touch(filename string) error {
	file, err := os.Create(path.Join(r.Dir(), filename))

	if os.IsExist(err) {
		return nil
	} else if err != nil {
		return err
	}

	return file.Close()
}

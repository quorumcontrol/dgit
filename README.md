## Decentragit helper
Implements a [git remote helper](https://git-scm.com/docs/git-remote-helpers) that stores repo information in a ChainTree.


### Usage
Protocol is registered as `dgit`, so origin should look like:
`git remote add origin dgit://quorumcontrol/tupelo`

Replacing `quorumcontrol/tupelo` with any repo name

Then proceed with normal git commands

### Installation
`go build -o git-remote-dgit && mv mv git-remote-dgit $GOPATH/bin/`

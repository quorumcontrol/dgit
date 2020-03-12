## dgit remote helper
Implements a [git remote helper](https://git-scm.com/docs/git-remote-helpers) that stores repo information in a ChainTree.

### Usage
Protocol is registered as `dgit`, so origin should look like:
`git remote add origin dgit://quorumcontrol/dgit`

Replacing `quorumcontrol/dgit` with any repo name

Then proceed with normal git commands

### Building
- Clone this repo.
- Run `make`. Generates `./dgit` in top level dir.

### Installation
- Run `make install`. Copies `dgit` and `git-remote-dgit` to your $GOPATH/bin dir, so add that to your path if necessary.

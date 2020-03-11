module github.com/quorumcontrol/decentragit-remote

go 1.13

replace (
	github.com/NebulousLabs/go-skynet => github.com/quorumcontrol/go-skynet v0.0.0-20200310210059-d794340c029a
	github.com/go-critic/go-critic => github.com/go-critic/go-critic v0.4.0
	github.com/golangci/errcheck => github.com/golangci/errcheck v0.0.0-20181223084120-ef45e06d44b6
	github.com/golangci/go-tools => github.com/golangci/go-tools v0.0.0-20190318060251-af6baa5dc196
	github.com/golangci/gofmt => github.com/golangci/gofmt v0.0.0-20181222123516-0b8337e80d98
	github.com/golangci/gosec => github.com/golangci/gosec v0.0.0-20190211064107-66fb7fc33547
	github.com/golangci/lint-1 => github.com/golangci/lint-1 v0.0.0-20190420132249-ee948d087217
	github.com/keybase/go-keychain => github.com/99designs/go-keychain v0.0.0-20191008050251-8e49817e8af4
	golang.org/x/xerrors => golang.org/x/xerrors v0.0.0-20191204190536-9bdfabe68543
	mvdan.cc/unparam => mvdan.cc/unparam v0.0.0-20190209190245-fbb59629db34
)

require (
	github.com/99designs/keyring v1.1.4
	github.com/NebulousLabs/go-skynet v0.0.0-20200306163948-8394a3e261ba
	github.com/ethereum/go-ethereum v1.9.3
	github.com/ipfs/go-bitswap v0.1.9-0.20191015150653-291b2674f1f1
	github.com/ipfs/go-datastore v0.4.4
	github.com/ipfs/go-ds-flatfs v0.4.0
	github.com/ipfs/go-ipfs-blockstore v0.1.0
	github.com/ipfs/go-ipld-format v0.0.2
	github.com/ipfs/go-log v1.0.2
	github.com/quorumcontrol/chaintree v1.0.2-0.20200124091942-25ceb93627b9
	github.com/quorumcontrol/messages/v2 v2.1.3-0.20200129115245-2bfec5177653
	github.com/quorumcontrol/tupelo-go-sdk v0.6.0-beta1.0.20200306010523-39fc767b60a1
	github.com/stretchr/testify v1.4.0
	go.uber.org/multierr v1.5.0 // indirect
	go.uber.org/zap v1.14.0
	golang.org/x/lint v0.0.0-20200130185559-910be7a94367 // indirect
	golang.org/x/tools v0.0.0-20200226224502-204d844ad48d // indirect
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15
	gopkg.in/src-d/go-billy.v4 v4.3.2
	gopkg.in/src-d/go-git-fixtures.v3 v3.5.0
	gopkg.in/src-d/go-git.v4 v4.13.1
	honnef.co/go/tools v0.0.1-2020.1.3 // indirect
)

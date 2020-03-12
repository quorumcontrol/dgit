package clientbuilder

import (
	"context"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/ipfs/go-bitswap"
	ds "github.com/ipfs/go-datastore"
	dsync "github.com/ipfs/go-datastore/sync"
	flatfs "github.com/ipfs/go-ds-flatfs"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	logging "github.com/ipfs/go-log"
	tupelo "github.com/quorumcontrol/tupelo-go-sdk/gossip/client"
	"github.com/quorumcontrol/tupelo-go-sdk/gossip/client/pubsubinterfaces/pubsubwrapper"
	"github.com/quorumcontrol/tupelo-go-sdk/gossip/types"
	"github.com/quorumcontrol/tupelo-go-sdk/p2p"
)

const ngToml = `
id = "gossip4"

BootstrapAddresses = [
  "/ip4/52.88.225.180/tcp/34001/ipfs/16Uiu2HAmQgHD5eqxDskKe21ythvG2T9o5i521kEdLrdgjc94sgCr",
  "/ip4/15.188.248.188/tcp/34001/ipfs/16Uiu2HAmNBupyDCfGSqo6ypNUmpHbYWy4jSaTBsbz6uRnsnY3JZN",
]

[[signers]]
VerKeyHex = "0x02bef6fc2cd935f83a19e07ef17e3561ed44f5fccbf3a8f74007dba7984922ae7f4d0f869a798c430d942d66c2446db3336cd8b8c5db86e08492417169b766ff2513fb7a57c3aea95b3291eb0c38ffcb84d8bafc502dd602d554edbf4e42323a3f2ef45476e6136d84e3307beaa1d505005396ec188394321895da27e7a015f5"
DestKeyHex = "0x04663234f326da31b9ab32a99e41cd399b415f9a55d989f2c9e2b338d1cba0b61196d6706b17826a45f26cd1eb9be2ae131fcf98d1c72547e450a9dc5af709eb0f"

[[signers]]
VerKeyHex = "0x8039afab271b89dd22180602133efa884fbe090249c1832925ff69ce9f5b647147299f5585f6d7b857b23670015250ba60a456c11ae1927f8c38b6f8ac9f8d6b5b5dc37648c61f7b4a95ad14e8967ddf898cca7bb28445893885c0c1030fc57a842e2cbcd6d03689691f58fe202ada5a1bfe6e7e01de149a4514f29e9a2c349e"
DestKeyHex = "0x04dc48195fc5af3c611fa8b28e8d4c0adba8deaf720ef07339abfe4022847d711cccf59d170f93357b0916b39f7045662000e004472daea5b0233c89c64020bf24"

[[signers]]
VerKeyHex = "0x47e9f5650766b3ce6519c45f515d18ec225ae2191ca9350d0f0993efe781bc1703c718a8ac42a4806b1df641aab90b78a0e6f97d036b445c6290724340c72e027564c6aa1b56e3735e92e460fa0508c861b89e906de5cffd537ae04c42dd89837710040ddce44df0cf9ec7bcacaa7dd1e630ffe88d75d612660419e299e36830"
DestKeyHex = "0x04e97151be30893132f2670ddb1ca8c2bdc586b3252a96d8ef42623f09f7159f9f0d48041d9f11e38d35218d8d10deba95e41ba58eb68b8034c8fc05cf05ea31ca"

[[signers]]
VerKeyHex = "0x760bc51e4cb173a46487f447961e9ed226798f4ef52e4c2a23fecc1f74f65a1558b669d5551f308ec7ffd7654c0aa223c39055ce086c3d6a5796b360aedd7c3862c71bdccc69ad6fc103a680ce5c41519a6dbbcc6f2c27e1a39f41159de350bb6d06c2b36651f647334a06036607b8c264e02d9913764e4b0002ba5ee01a8c7c"
DestKeyHex = "0x04d75251a9db8182e63a2f1483bbc240e18ca232de33207ee704275b03f7c419f5807632c8dd291589349dafce86c0e388f2f669ba9f591fdcadbe03192b72aa3c"

[[signers]]
VerKeyHex = "0x77b685350b31d22b50d991cf29d00e8f22a7b22039fcb63264a06503d01797e0825c9dd78828a7eea68543ff309d9195817746b805e5a241bac25599aa480e7182b0dd479934e5f5bb2b58887363ddd68e67fb8cda4811c565cd585f75aaf86171b6283a384c40c8de2798dd107a1b491fe5f5ec5051f16f1d7d4f37fc7f54fa"
DestKeyHex = "0x04a3ff0167eeecbdaf3f4df25acabca8d18f6f138e60daf2e96b9db0edbc70eee8054ce211e950ecf2b19c9d8db92c3b205a62e8bfd9b68f35a26a5e4343a18fa2"
`

const localNgToml = `
id = "tupelolocal"
BootstrapAddresses = [
    "/ip4/127.0.0.1/tcp/34001/ipfs/16Uiu2HAm3TGSEKEjagcCojSJeaT5rypaeJMKejijvYSnAjviWwV5"
]
[[signers]]
VerKeyHex = "0x15796b266a7d6b7c6b29c5bf97ad376fe8457e4d56bb0612ec8703c65ca7b6bb5dca004d55f5238d7764cd100c9e9cac3c5abce902bae8a5f9c29de716a145595b071b1b7038a48b4f6f88e7664b38c02062f64b3ceb499e4cbb82361457dcd731f5b48901871e7fd56a9c91ab3e06d3f7cb27288962686d9a05e02c1482f01f"
DestKeyHex = "0x04f6dee3f7da1da58afd6ee58ea6b858fb67664fc6e2240bb6e3a75c0e1db9bbef5f413c8604bb864513d3cf27eca60b539b048b2a08f8799570c14dfb73f3f391"

[[signers]]
VerKeyHex = "0x7be8c92c8c295ef3e97be28f469f5f94d10ee7db4d202776bee5cf55c62d508a0c3550a19342d768ff073c0798ce003646df586ef588a9e9443a0ca86a234ed15150dc98ecc3f1071649fca03426f1c8c215a90752f51faa3e2e788e1dae2e9e5cf87c1ca4239a0949a0ba6ea09c061a538372cc4230dedafae929b170ad7704"
DestKeyHex = "0x0438b196bddb9c3ec395b8ccb07bdab44ec768c084e7141b09ac5638d47fffbd5e7b7623f499a2e714e31464a356a0e30ad7c93045b6cd9957b45e957cc15dcb99"

[[signers]]
VerKeyHex = "0x88aefad94805db01cacaf190f47bc9e40f584b5085c651da168ac4034d570b4750bf7b23803d204e483e407a5ca34ee7f7a434733346451cf3f5d26c0d11e5ac45398a03fbba2d3b0dfc21cdf14615430cea394bd9423d8527eaa82a96aa6d20655724d99770ee3488b6537d6be143b84b21ad5ee12c190048757fe453313fd2"
DestKeyHex = "0x0468924bd1341b5cec1fed888aaf1e3caa94e7d0f13d4f4573b01b296374b9e710a58a7b40e7161c0bcf7fd41832441ca21076f3846e854c8d8c640f2469a552b1"
`

func BuildLocal(ctx context.Context) (*tupelo.Client, *p2p.BitswapPeer, error) {
	logging.SetAllLoggers(logging.LevelFatal)

	ngConfig, err := types.TomlToConfig(localNgToml)
	if err != nil {
		return nil, nil, err
	}
	return BuildWithConfig(ctx, &Config{
		Storage:           dsync.MutexWrap(ds.NewMapDatastore()),
		NotaryGroupConfig: ngConfig,
	})
}

func Build(ctx context.Context, storagePath string) (*tupelo.Client, *p2p.BitswapPeer, error) {
	if err := os.MkdirAll(storagePath, 0755); err != nil {
		return nil, nil, err
	}

	storage, err := flatfs.CreateOrOpen(path.Join(storagePath, "storage"), flatfs.NextToLast(2), true)
	if err != nil {
		return nil, nil, err
	}

	return BuildWithConfig(ctx, &Config{Storage: storage})
}

type Config struct {
	NotaryGroupConfig *types.Config
	Storage           ds.Batching
}

func BuildWithConfig(ctx context.Context, config *Config) (*tupelo.Client, *p2p.BitswapPeer, error) {
	var err error

	ngConfig := config.NotaryGroupConfig
	if ngConfig == nil {
		ngConfig, err = types.TomlToConfig(ngToml)
		if err != nil {
			return nil, nil, err
		}
	}

	storage := config.Storage

	blockstore.BlockPrefix = ds.NewKey("")
	bs := blockstore.NewBlockstore(storage)
	bs = blockstore.NewIdStore(bs)

	p2pHost, peer, err := p2p.NewHostAndBitSwapPeer(
		ctx,
		p2p.WithDiscoveryNamespaces(ngConfig.ID),
		p2p.WithBitswapOptions(bitswap.ProvideEnabled(false)), // maybe this should be true if there is a long running dgit node
		p2p.WithBlockstore(bs),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating host: %w", err)
	}

	_, err = p2pHost.Bootstrap(ngConfig.BootstrapAddresses)
	if err != nil {
		return nil, nil, fmt.Errorf("error bootstrapping: %w", err)
	}

	group, err := ngConfig.NotaryGroup(nil)
	if err != nil {
		return nil, nil, fmt.Errorf("error getting notary group: %v", err)
	}

	if err = p2pHost.WaitForBootstrap(1+len(group.Signers)/2, 15*time.Second); err != nil {
		return nil, nil, err
	}

	cli := tupelo.New(group, pubsubwrapper.WrapLibp2p(p2pHost.GetPubSub()), peer)
	err = cli.Start(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("error starting client: %v", err)
	}

	return cli, peer, nil
}

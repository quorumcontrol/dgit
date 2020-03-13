package remotehelper

var MsgWelcome = `
Welcome to dgit!

A private key has been generated for you and is stored in %s.

Below is your dgit public address, share this with others to gain write access to dgit repos.

%s
`

var MsgRepoNotFound = `
Repository does not exist at %s

You can create a dgit repository by doing a 'git push'
`

var KeyringPrettyNames = map[string]string{
	"*keyring.keychain":       "macOS Keychain Access",
	"*keyring.kwalletKeyring": "KWallet (KDE Wallet Manager)",
	"*keyring.windowsKeyring": "Windows Credential Manager",
	"*keyring.secretsKeyring": "libsecret",
	"*keyring.passKeyring":    "pass",
}

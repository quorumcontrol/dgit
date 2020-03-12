package msg

var Welcome = `
Welcome to dgit!

A private key has been generated for you and is stored in %s.

Below is your dgit public address, share this with others to gain write access to dgit repos.

%s
`

var RepoNotFound = `
Repository does not exist at %s

You can create a dgit repository by doing a 'git push'
`

var RepoNotFoundInPath = `
No .git directory found in {{.path}}.

Please change directories to a git repo and run '{{.cmd}}' again.

If you would like to create a new repo, use 'git init' normally and run '{{.cmd}}' again.
`

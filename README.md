[![License](http://img.shields.io/:license-mit-blue.svg?style=flat-square)](http://badges.mit-license.org)
[![Contributor Covenant](https://img.shields.io/badge/Contributor%20Covenant-v2.0%20adopted-ff69b4.svg)](CODE_OF_CONDUCT.md)

<!-- PROJECT LOGO -->
<br />
<p align="center">
  <a href="https://github.com/quorumcontrol/dgit">
    <img src="dgit-black.png" alt="Logo" width="150" height="125">
  </a>

  <h3 align="center">dgit</h3>

  <p align="center">
    <b>dgit</b> is an opensource project built by Quorum Control which combines
    the power of <br>git, the Tupelo DLT and Skynet from Sia.  <br>
    <b>dgit</b> uses decentralized ownership and storage to make it trivial to
    create a decentralized mirror of your project.<br>
    <b>dgit</b> accomplishes this without changing your github workflow.<br>
  </p>
</p>

### Built With

* [Git](https://git-scm.com/)
* [Tupelo DLT](https://docs.tupelo.org/)
* [Skynet](https://siasky.net/)

<!-- TABLE OF CONTENTS -->
## Table of Contents

- [Getting Started](#getting-started)
  - [Installation](#installation)
  - [Usage](#usage)
  - [Building](#building)
- [Contributing](#contributing)
- [License](#license)
- [Contact](#contact)


<!-- GETTING STARTED -->
## Getting Started
With three simple steps you can create a decentralized mirror of your existing github project.
All changes will be automatically propogated to the mirror version and the git services you depend on will be there when you need them.

### Installation
- Run `make install`. Copies `git-remote-dgit` to your $GOPATH/bin dir, so add that to your path if necessary.

### Usage
Protocol is registered as `dgit`, so origin should look like:
`git remote add origin dgit://quorumcontrol/tupelo`

Replacing `quorumcontrol/tupelo` with any repo name.

Then proceed with normal git commands.

### Building
- Clone this repo.
- Run `make`. Generates `./git-remote-dgit` in top level dir.

<!-- CONTRIBUTING -->
## Contributing

Contributions are what make the open source community such an amazing place to be learn, inspire, and create. Any contributions you make are **greatly appreciated**.

1. Fork the Project
2. Create your Feature Branch (`git checkout -b feature/AmazingFeature`)
3. Commit your Changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the Branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

<!-- LICENSE -->
## License

Distributed under the MIT License. See `LICENSE` for more information.

<!-- CONTACT -->
## Contact

Hop into our developer chat on Telegram: https://t.me/joinchat/IhpojEWjbW9Y7_H81Y7rAA

Project Link: [https://github.com/quorumcontrol/dgit](https://github.com/quorumcontrol/dgit)

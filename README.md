opendex-launcher
================

[![Discord](https://img.shields.io/discord/628640072748761118.svg)](https://discord.gg/RnXFHpn)
[![License: AGPL v3](https://img.shields.io/badge/License-AGPL%20v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)

The opendex-launcher is a thin wrapper of opendexd-docker launcher which enables running any branch of opendexd-docker since version 2. It will keep a low update frequency, and it will be embedded in our GUI and CLI applications. 

### Build

On *nix platform
```sh
make
```

On Windows platform
```
mingw32-make
```

### Run

On *nix platform
```sh
export BRANCH=master
export NETWORK=mainnet
./opendex-launcher setup
```

On Windows platform (with CMD)
```
set BRANCH=master
set NETWORK=mainnet
./opendex-launcher setup
```

On Windows platform (with Powershell)
```
$Env:BRANCH = "master"
$Env:NETWORK = "mainnet"
./opendex-launcher setup
```

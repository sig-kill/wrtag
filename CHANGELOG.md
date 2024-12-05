# Changelog

## [0.5.0](https://www.github.com/sentriz/mrtag/compare/v0.4.0...v0.5.0) (2024-12-05)


### Features

* **ci:** use matrix to build binaries ([938ae37](https://www.github.com/sentriz/mrtag/commit/938ae379056646a4f3801405d136b7d8273e34f1))

## [0.4.0](https://www.github.com/sentriz/mrtag/compare/v0.3.0...v0.4.0) (2024-12-05)


### Features

* **ci:** don't use qemu for multi platform builds ([b4b90c0](https://www.github.com/sentriz/mrtag/commit/b4b90c08eeedcd500c7a0961759d4b9798cb1e81))

## [0.3.0](https://www.github.com/sentriz/mrtag/compare/v0.2.2...v0.3.0) (2024-12-01)


### ⚠ BREAKING CHANGES

* rebrand to mrtag
* rename `wrtagsync` -> `wrtag sync`

### Features

* rebrand to mrtag ([a8399af](https://www.github.com/sentriz/mrtag/commit/a8399af5452f037689d1f66ad57907541c1d9a93)), closes [#58](https://www.github.com/sentriz/mrtag/issues/58)
* rename `wrtagsync` -> `wrtag sync` ([a3c097f](https://www.github.com/sentriz/mrtag/commit/a3c097f1197d4e63780c0b66be08a8c3ff7c379c))

### [0.2.2](https://www.github.com/sentriz/mrtag/compare/v0.2.1...v0.2.2) (2024-11-28)


### Bug Fixes

* **ci:** upload binaries to output tag ([c0b5677](https://www.github.com/sentriz/mrtag/commit/c0b5677b9b077cc2c710d5712f2b3531a377bf4f))

### [0.2.1](https://www.github.com/sentriz/mrtag/compare/v0.2.0...v0.2.1) (2024-11-28)


### Bug Fixes

* **ci:** don't use hardcoded binary names ([c9a80b2](https://www.github.com/sentriz/mrtag/commit/c9a80b2be3d4f2ee38e932169ab2701fd6983584))

## [0.2.0](https://www.github.com/sentriz/mrtag/compare/v0.1.0...v0.2.0) (2024-11-28)


### Features

* **ci:** faster binary build ([696eb83](https://www.github.com/sentriz/mrtag/commit/696eb838bdd2a5608359a475faa80f3c28c740e8))

## 0.1.0 (2024-11-28)


### ⚠ BREAKING CHANGES

* **mrtagweb:** replace bolt with sqlite

### Features

* **ci:** add binaries ([dcf0424](https://www.github.com/sentriz/mrtag/commit/dcf042458978ec0743e79b8b43abb0759e61ab49))
* **clientutil:** log with ctx ([814372a](https://www.github.com/sentriz/mrtag/commit/814372ac47c3e8847634d21e3bdaab753499cf96))
* use go.senan.xyz/taglib-wasm ([5318e65](https://www.github.com/sentriz/mrtag/commit/5318e65c4a1ebb386e442c2056eae9304b5ffaab))
* **mrtag:** validate situations where tracks can't be sorted before matching ([20c616a](https://www.github.com/sentriz/mrtag/commit/20c616a13e5f112a88e42c724f545534a2279393)), closes [#52](https://www.github.com/sentriz/mrtag/issues/52)
* **mrtagweb:** enforce db path ([a6bf28f](https://www.github.com/sentriz/mrtag/commit/a6bf28f8ae4a8917abc24ee34d966b519d1a8358))
* **mrtagweb:** replace bolt with sqlite ([26e6889](https://www.github.com/sentriz/mrtag/commit/26e688999e252ca5c15eb4c14433319e4b0ae195))


### Bug Fixes

* **metadata:** adjust help output ([76568c5](https://www.github.com/sentriz/mrtag/commit/76568c5ed8382647a3ede5ce9421c85b8cd4a33c))
* **tag:** bump go-taglib-wasm ([cdfb74c](https://www.github.com/sentriz/mrtag/commit/cdfb74ca3453139ec471c236b244c56c353a57ab))

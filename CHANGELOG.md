# Changelog

## 0.1.0 (2026-03-27)

This is the first release of plumber. Things will change, and until those pending changes have been addressed plumber will have a major version of 0. Things will break.

### Features

* add an init command ([7051bdf](https://github.com/gmc-norr/plumber/commit/7051bdfae123c8e902b9437f569b92168a445c83))
* add handling of python virtual environments ([798f767](https://github.com/gmc-norr/plumber/commit/798f767f60075e5995e97b0af615020cc04d80ab))
* add snakemake support ([a90e4b8](https://github.com/gmc-norr/plumber/commit/a90e4b883587e2874e7bf87ae207b914e77b2c10))
* allow for a local config file repo ([#2](https://github.com/gmc-norr/plumber/issues/2)) ([e103327](https://github.com/gmc-norr/plumber/commit/e1033278348d62b5e7c74021906d31a96e7de968))
* better ergonomics for nextflow run ([#3](https://github.com/gmc-norr/plumber/issues/3)) ([348cdc8](https://github.com/gmc-norr/plumber/commit/348cdc8440acd483e3cb2b5a463d55f7a0d31505))
* better webhook messages on pipeline error ([#26](https://github.com/gmc-norr/plumber/issues/26)) ([06f901c](https://github.com/gmc-norr/plumber/commit/06f901c8e62821fc9589854ae801eb420a9f8c66))
* plumberfile implementation ([1bc505a](https://github.com/gmc-norr/plumber/commit/1bc505af17b66122fb5309a1864a621de97cad13))
* prettier output from config list ([a12a5dc](https://github.com/gmc-norr/plumber/commit/a12a5dc51d5b611a06a9c3526b4281c8e9dd430d))
* remove intermediate nextflow files on successful execution ([#14](https://github.com/gmc-norr/plumber/issues/14)) ([a693649](https://github.com/gmc-norr/plumber/commit/a693649f8de55139a92974b1c30b1c2afcf6cc34))
* remove pipeline GitHub check ([#7](https://github.com/gmc-norr/plumber/issues/7)) ([eaabf94](https://github.com/gmc-norr/plumber/commit/eaabf94fc76440c8223e0c2ff0a64bce8695bad4))
* richer config CLI command ([7cc20a0](https://github.com/gmc-norr/plumber/commit/7cc20a0bf16e5d5cfa0cbf5c7a9c0e7a34872e60))
* send messages to webhooks ([#15](https://github.com/gmc-norr/plumber/issues/15)) ([57174b4](https://github.com/gmc-norr/plumber/commit/57174b443adec3563150e3646193821587c6e97f))
* update plumberfile structure ([9d59b64](https://github.com/gmc-norr/plumber/commit/9d59b640b07eb14995273c8dfda007ab141c94ab))
* use analysis IDs ([#35](https://github.com/gmc-norr/plumber/issues/35)) ([54adf62](https://github.com/gmc-norr/plumber/commit/54adf620df94eb18ca41a3c84cb51a5aaecc46e7))
* use current working directory as default working directory for hydra runs ([54adf62](https://github.com/gmc-norr/plumber/commit/54adf620df94eb18ca41a3c84cb51a5aaecc46e7))


### Bug Fixes

* add NEXTFLOW_CONFIG_HOME env var for nextflow runs ([#32](https://github.com/gmc-norr/plumber/issues/32)) ([3146564](https://github.com/gmc-norr/plumber/commit/31465649a4a22c3e416046d72853fec60a87e61a))
* bind all persistent flags to viper ([#38](https://github.com/gmc-norr/plumber/issues/38)) ([f16d42d](https://github.com/gmc-norr/plumber/commit/f16d42d4f97bc1c564304d1fa2b6fcdd84e4c2fb))
* fetching a single config still fetched all ([177f033](https://github.com/gmc-norr/plumber/commit/177f03352a676777e34e98ed412362a8076745a1))
* hint about all versions when downloading missing version ([77bf967](https://github.com/gmc-norr/plumber/commit/77bf967b4da7677162922370eab8338b129a0bce))
* incorrect config repo variables for nextflow run ([8df193a](https://github.com/gmc-norr/plumber/commit/8df193aec0f4a33bc695b5a1a959e4086072dde3))
* **nextflow:** pass pipeline argument before options ([06ed8a3](https://github.com/gmc-norr/plumber/commit/06ed8a3c2a2c7b50c59a0fbde4e15e9f7824ebb0))
* rename PLUMBER_ASSETS_PATH -&gt; PLUMBER_PIPELINE_ASSETS ([6733fdc](https://github.com/gmc-norr/plumber/commit/6733fdc0b2a727a44cb28161a61beb8664113d8e))
* use absolute path to working directory for hydra runs ([54adf62](https://github.com/gmc-norr/plumber/commit/54adf620df94eb18ca41a3c84cb51a5aaecc46e7))

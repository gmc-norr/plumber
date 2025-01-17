# plumber

![go build and test](https://github.com/gmc-norr/plumber/actions/workflows/go.yml/badge.svg)
![golangci-lint](https://github.com/gmc-norr/plumber/actions/workflows/golangci-lint.yaml/badge.svg)

---

Plumber is a wrapper for Snakemake and nf-core pipelines that are run at GMC Norr.

## Requirements

- git
- nextflow

## Running plumber

Plumber is very much adapted to work with the [gmc-norr/config-files](https://github.comg/gmc-norr/config-files) repository. Config files will be downloaded for local use, and the default location for this is `${XDG_CONFIG_HOME}/plumber`. If `XDG_CONFIG_HOME` is not set, `$HOME/.config/plumber` will be used instead.

## Environment variables

- `PLUMBER_CONFIG_HOME`: Location where config files will be stored. Defaults to `${XDG_CONFIG_HOME}/plumber`, and if that is undefined `$HOME/.config/plumber`.
- `PLUMBER_LOGLEVEL`: Controls verbosity of logs. Valid values are `debug`, `info`, `warn` and `error`. These are listed in decreasing level of verbosity. Default is `warn`.

## Plumberfiles

Plumber makes use of simple yaml metadata files called plumberfiles.
These come in two flavours: one representing a collection of pipeline configurations and one representing the config for a single pipeline.
They both have the same format.
The difference is that the one representing a single pipeline has fields defining the origin of the configuration (a git repo) and what revision of that configuration was used.
Another difference is that while plumberfile representing a collection of pipiline can contain one or more pipelines, the version representing a single pipeline can contain only one pipeline configuration.

Plumber ships with a command for validating the format of a plumber file:

```bash
plumber config validate plumber.yaml
```

## Current limitations

At the moment, only support for Nextflow pipelines has been implemented. More is to come.

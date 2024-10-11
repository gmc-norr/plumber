# plumber

Plumber is a wrapper for Snakemake and nf-core pipelines that are run at GMC Norr.

## Requirements

- git

## Running plumber

Plumber is very much adapted to work with the [gmc-norr/config-files](https://github.comg/gmc-norr/config-files) rrepository. Config files will be cloned for local use, and the default location for this is `${XDG_CONFIG_HOME}/plumber`. If `XDG_CONFIG_HOME` is not set, `$HOME/.config/plumber` will be used instead.

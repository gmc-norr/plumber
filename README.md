# plumber

Plumber is a wrapper for Snakemake and nf-core pipelines that are run at GMC Norr.

## Requirements

- git
- nextflow

## Running plumber

Plumber is very much adapted to work with the [gmc-norr/config-files](https://github.comg/gmc-norr/config-files) rrepository. Config files will be cloned for local use, and the default location for this is `${XDG_CONFIG_HOME}/plumber`. If `XDG_CONFIG_HOME` is not set, `$HOME/.config/plumber` will be used instead.

In order to do some sanity checks of pipelines, the Github API is being used. Without an authorisation token, it is quite easy to hit the rate limit of the API. The program will still work, but these checks will not be performed. By generating an access token and defining the environment variable `PLUMBER_GITHUB_TOKEN` authorisation will be performed, and higher rate limits will be set.

## Environment variables

- `PLUMBER_CONFIG_HOME`: Location where config files will be stored. Defaults to `${XDG_CONFIG_HOME}/plumber`, and if that is undefined `$HOME/.config/plumber`.
- `PLUMBER_GITHUB_TOKEN`: Github access token for API access.
- `PLUMBER_LOGLEVEL`: Controls verbosity of logs. Valid values are `debug`, `info`, `warn` and `error`. These are listed in decreasing level of verbosity. Default is `warn`.

## Current limitations

At the moment, only support for Nextflow pipelines has been implemented. More is to come.

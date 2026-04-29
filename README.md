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

When an analysis is started, the file `.plumber-analysis.json` will be written in the working directory for the analysis. This contains information about the pipeline that is being run, and also includes an analysis ID. If an analysis is started in a working directory where this file already exists, plumber checks that the ID is the same as the one being supplied on the command line. If this is not the case, plumber exits with an error.

## Environment variables

- `PLUMBER_CONFIG_HOME`: Location where config files will be stored. Defaults to `${XDG_CONFIG_HOME}/plumber`, and if that is undefined `$HOME/.config/plumber`.
- `PLUMBER_LOGLEVEL`: Controls verbosity of logs. Valid values are `debug`, `info`, `warn` and `error`. These are listed in decreasing level of verbosity. Default is `warn`.
- `PLUMBER_WEBHOOK_URL`: URL to send webhook messages to.
- `PLUMBER_WEBHOOK_API_KEY`: API key for the webhook endpoint (if needed).
- `PLUMBER_WEBHOOK_NO_VERIFY`: Don't do TLS verification for the webhook requests.
- `PLUMBER_CERTS`: Path to TLS certificates needed for the webhooks.

## Webhooks

Plumber can send progress messages to a webhook endpoint. This can be configured either on the command line or by environment variables. If a URL is set, the feature will be enabled.

Messages are sent as JSON with the following structure:

```txt
{
    "analysis_id": ID of the analysis,
    "pipeline": name of the pipeline,
    "pipeline_version": pipelinen version,
    "workdir": working directory for the execution,
    "message": a text message or an object,
    "message_type": one of "init", "start", "end", "progress",
    "success": true if associated with a successful step, false otherwise,
    "error": the error encountered if success is false, otherwise null
    "time": time when the message was sent
}
```

## Plumberfiles

Plumber makes use of simple yaml metadata files called plumberfiles.
These come in two flavours: one representing a collection of pipeline configurations and one representing the config for a single pipeline.
They both have the same format, but there are a couple of differences.
Firstly, a plumberfile representing a single pipeline has fields defining the origin of the configuration (a git repo) and what revision of that configuration was used.
Secondly, a plumberfile representing a collection of pipelines can contain one or more pipelines, the version representing a single pipeline must contain one, and only one, pipeline configuration.

Plumber ships with a command for validating the format of a plumber file:

```bash
plumber config validate plumber.yaml
```

## Webhook test server

Plumber comes with a simple server for testing the webhook functionality. Easiest is to run this with

```bash
go run ./cmd/webhookserver
```

This will listen for POST requests on localhost:3000 by default, so it can be spun up in one terminal session, and in another session you can do

```bash
plumber run <pipeline> [options] --webhook-url http://localhost:3000
```

The server expects a JSON body in the request, and will just print the payload for each request.
Auth can also be enabled by setting `--api-key` and `--api-key-header` for the test server.
Plumber must then be run with `--webhook-api-key` in order for the requests to be successful.

For example:

```bash
# session 1
go run ./cmd/webhookserver --api-key secret --api-key-header api-key

# session 2
plumber run <pipeline> [options] --webhook-url http://localhost:3000 --webhook-api-key "api-key=secret"
```

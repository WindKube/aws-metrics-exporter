# aws-metrics-exporter

A Go application that exposes AWS objects to the backend for inventory analysis.

It scrapes every configured AWS account on a schedule, assuming a read-only role in each one, and
writes the AWS JSON representation of each object into a daily index in the storage backend.
Temporal splits the work so one account, one resource type and one region is a single retryable
unit.

## Supported resource types

| Type | Scope | AWS calls |
|---|---|---|
| `aws_iam_user` | global, scraped once per account | `ListUsers` plus tags, access keys, MFA devices, attached and inline policies, groups |
| `aws_elasticache_replication_group` | regional, scraped once per configured region | `DescribeReplicationGroups`, `ListTagsForResource` |

## Backends

Elasticsearch only. Documents land in `<index_prefix>-<resource_type>-<YYYY.MM.DD>` with
`_id = <account_id>:<region>:<resource_type>:<resource_id>`, so a re-run overwrites rather than
duplicates and each daily index is a snapshot of the estate.

```json
{
  "@timestamp": "2026-09-18T01:00:00Z",
  "resource": { "type": "aws_iam_user", "id": "arn:aws:iam::111111111111:user/kw", "name": "kw" },
  "cloud": { "provider": "aws", "region": "global", "account": { "id": "111111111111", "name": "prod" } },
  "aws": { "UserName": "kw", "...": "the AWS object as the SDK returns it" }
}
```

The index template in [`deploy/elasticsearch/`](deploy/elasticsearch/README.md) has to exist before
the first scrape; the exporter never creates it and only needs write access to `<index_prefix>-*`.

Adding a backend means implementing `storage.Store` in `internal/storage`.

## Configuration

See [`configs/example.yaml`](configs/example.yaml). Credentials are never read from the file:

| Variable | Purpose |
|---|---|
| `AWSME_STORAGE_ELASTICSEARCH_USERNAME` / `_PASSWORD` / `_API_KEY` | Elasticsearch credentials |
| `SENTRY_DSN`, `SENTRY_ENVIRONMENT` | Error tracking; Sentry is off when the DSN is unset |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Tracing; the OTLP exporter is off when unset |

Any config key can be overridden with `AWSME_` plus the key path in upper case, for example
`AWSME_GLOBAL_LOG_LEVEL=debug`.

Set `global.temporal.tls.enabled` to connect over TLS; `cert_file` and `key_file` add a client
certificate for mTLS, `ca_file` trusts a private CA instead of the system store and
`insecure_skip_verify` drops server certificate verification entirely.

Each account's `regions` list applies only to regional resource types; global ones are scraped once
per account regardless. An account inherits `global.regions`, `resources` and
`global.scrape_interval` unless it sets its own.

## Running

```
aws-metrics-exporter worker      Temporal worker, schedule reconciliation and health probes
aws-metrics-exporter scrape      scrape one account directly, without Temporal
aws-metrics-exporter schedule    apply | list | pause | resume | trigger
aws-metrics-exporter config      show | validate
```

The `worker` command reconciles one Temporal Schedule per account (`ame-acct-<name>`) on start:
accounts added to the config get a schedule, accounts removed lose theirs. Pause, resume, backfill
and trigger are available per account without touching the others.

```bash
task compose:up                                        # elasticsearch, kibana, temporal
task es:template                                       # create the index template
task config                                            # validate the config file
task scrape -- --account prod --resource aws_iam_user  # one-shot, useful for checking IAM access
task worker
curl -s localhost:8080/readyz
```

`/readyz` and `/healthz` return 503 once two consecutive probes of the storage backend or Temporal
fail, which takes the pod out of rotation in Kubernetes. `/livez` reports only that the process is
up.

## AWS access

The exporter assumes `role_arn` in each configured account.
[`terraform/aws/`](terraform/aws/README.md) creates that role across an organization with a
CloudFormation stack set, plus the hub role the exporter itself runs as.

## Releases

Commits on `main` follow [Conventional Commits](https://www.conventionalcommits.org). Release Please
keeps a release PR open with the next version and changelog; merging it tags the release and, in the
same workflow run, builds and publishes the image.

Images are `ghcr.io/windkube/aws-metrics-exporter`, built per architecture on a runner of that
architecture (`linux/amd64`, `linux/arm64`), pushed by digest and assembled into one manifest list.
Every release is Trivy-scanned, signed with keyless cosign and carries a GitHub build-provenance
attestation. Verify a pulled image with:

```bash
cosign verify \
  --certificate-identity-regexp '^https://github.com/WindKube/aws-metrics-exporter/\.github/workflows/' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  ghcr.io/windkube/aws-metrics-exporter:<version>

gh attestation verify oci://ghcr.io/windkube/aws-metrics-exporter:<version> --owner WindKube
```

> On the first release GHCR creates the package private and unlinked. Set it public and link it to
> the repository in the package settings.

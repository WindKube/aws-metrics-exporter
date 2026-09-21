# Terraform example

Creates the two halves of the exporter's AWS access:

- **Hub role** (`hub.tf`) — the role `aws-metrics-exporter` runs as, allowed to `sts:AssumeRole` on
  `arn:aws:iam::*:role/aws-metrics-exporter`. Its trust policy is passed in whole, so this module
  stays agnostic about where the exporter runs.
- **Member roles** (`stackset.tf`) — a service-managed CloudFormation stack set that creates the
  read-only `aws-metrics-exporter` role in every account of the targeted organizational units.
  `auto_deployment` is on, so accounts added to those OUs later get the role without a re-apply.

Apply from the organization management account, or from a delegated CloudFormation administrator
with `call_as = "DELEGATED_ADMIN"`.

```hcl
organizational_unit_ids    = ["ou-abcd-11111111"]
hub_role_trust_policy_json = data.aws_iam_policy_document.exporter_trust.json
```

```bash
terraform init
terraform apply
```

Then list each member account in the exporter's config:

```yaml
accounts:
  - name: prod
    id: "222222222222"
    role_arn: arn:aws:iam::222222222222:role/aws-metrics-exporter
```

## Management account

A service-managed stack set does not deploy into the organization management account. To scrape it
as well, create the same role there separately and point an account entry at it.

## Permissions

The member role's policy lives in `templates/member-role.yaml`. Adding a collector to the exporter
means adding its API calls there, otherwise the scrape fails with a non-retryable
`AwsPermissionDenied` activity error.

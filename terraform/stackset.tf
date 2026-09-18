resource "aws_cloudformation_stack_set" "member_role" {
  name             = var.member_role_name
  description      = "Read-only role assumed by aws-metrics-exporter in every member account."
  permission_model = "SERVICE_MANAGED"
  capabilities     = ["CAPABILITY_NAMED_IAM"]
  call_as          = var.call_as
  template_body    = file("${path.module}/templates/member-role.yaml")

  parameters = {
    RoleName   = var.member_role_name
    HubRoleArn = aws_iam_role.hub.arn
  }

  auto_deployment {
    enabled                          = true
    retain_stacks_on_account_removal = false
  }
}

resource "aws_cloudformation_stack_set_instance" "member_role" {
  stack_set_name = aws_cloudformation_stack_set.member_role.name
  region         = var.region
  call_as        = var.call_as

  deployment_targets {
    organizational_unit_ids = var.organizational_unit_ids
  }
}

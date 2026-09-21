resource "aws_iam_role" "hub" {
  name               = var.hub_role_name
  description        = "Identity aws-metrics-exporter runs as; assumes the per-account exporter roles."
  assume_role_policy = var.hub_role_trust_policy_json
}

data "aws_iam_policy_document" "assume_member_roles" {
  statement {
    effect    = "Allow"
    actions   = ["sts:AssumeRole", "sts:TagSession"]
    resources = ["arn:aws:iam::*:role/${var.member_role_name}"]
  }
}

resource "aws_iam_role_policy" "assume_member_roles" {
  name   = "assume-member-roles"
  role   = aws_iam_role.hub.id
  policy = data.aws_iam_policy_document.assume_member_roles.json
}

output "hub_role_arn" {
  description = "ARN of the role aws-metrics-exporter runs as."
  value       = aws_iam_role.hub.arn
}

variable "region" {
  description = "Region the stack set instances are created in. IAM is global, so one region is enough."
  type        = string
  default     = "eu-west-1"
}

variable "hub_role_name" {
  description = "Name of the role aws-metrics-exporter runs as."
  type        = string
  default     = "aws-metrics-exporter"
}

variable "hub_role_trust_policy_json" {
  description = "Assume-role policy of the hub role, describing whatever identity the exporter runs as."
  type        = string
}

variable "member_role_name" {
  description = "Name of the role created in every member account. Must match the role_arn suffix in the exporter config."
  type        = string
  default     = "aws-metrics-exporter"
}

variable "organizational_unit_ids" {
  description = "Organizational units whose accounts get the member role. Use the organization root id to cover everything."
  type        = list(string)
}

variable "call_as" {
  description = "SELF when running in the organization management account, DELEGATED_ADMIN from a delegated CloudFormation administrator."
  type        = string
  default     = "SELF"
}

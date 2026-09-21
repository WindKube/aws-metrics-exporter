variable "template_name" {
  description = "Name of the index template. Only its index pattern has to match the exporter's index_prefix, the name is free."
  type        = string
  default     = "aws-inventory"
}

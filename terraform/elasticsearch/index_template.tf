locals {
  index_template = jsondecode(file("${path.module}/../../deploy/elasticsearch/index-template.json"))
}

resource "elasticstack_elasticsearch_index_template" "inventory" {
  name           = var.template_name
  index_patterns = local.index_template.index_patterns
  priority       = local.index_template.priority

  template {
    mappings = jsonencode(local.index_template.template.mappings)
  }
}

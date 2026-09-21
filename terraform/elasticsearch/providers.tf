terraform {
  required_version = ">= 1.5"

  required_providers {
    elasticstack = {
      source  = "elastic/elasticstack"
      version = ">= 0.11"
    }
  }
}

# Connection details come from ELASTICSEARCH_ENDPOINTS and ELASTICSEARCH_API_KEY (or
# ELASTICSEARCH_USERNAME and ELASTICSEARCH_PASSWORD), so no credentials live in this module.
provider "elasticstack" {
  elasticsearch {}
}

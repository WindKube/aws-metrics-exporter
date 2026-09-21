# Terraform example

Applies [`deploy/elasticsearch/index-template.json`](../../deploy/elasticsearch/index-template.json)
to an Elasticsearch cluster. The JSON file stays the single source of the mapping, so the same
document is used whether the template is created with `curl`, Kibana or Terraform.

```bash
export ELASTICSEARCH_ENDPOINTS=https://my-cluster.es.eu-west-1.aws.cloud.es.io:443
export ELASTICSEARCH_API_KEY=...

terraform init
terraform apply
```

Username and password work as well, via `ELASTICSEARCH_USERNAME` and `ELASTICSEARCH_PASSWORD`. See
the [provider documentation](https://registry.terraform.io/providers/elastic/elasticstack/latest/docs)
for the other authentication options.

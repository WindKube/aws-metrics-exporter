# Elasticsearch index template

[`index-template.json`](index-template.json) maps the document envelope explicitly and the raw AWS
object as a single `flattened` field. Without it the first scrape of an IAM user with inline
policies dynamically maps every policy statement key and the index hits the 1000 field limit.

Create it before the first scrape. The exporter does not create or check it, so it needs no
privileges beyond writing into `<index_prefix>-*`.

```bash
curl -fsS -XPUT "$ES_URL/_index_template/aws-inventory" \
  -H 'Content-Type: application/json' \
  --data-binary @deploy/elasticsearch/index-template.json
```

Or in Kibana Dev Tools, `PUT _index_template/aws-inventory` with the file's contents as the body.
[`terraform/elasticsearch`](../../terraform/elasticsearch) applies the same file with the
`elasticstack` provider.

The template applies to `aws-inventory-*`. Change `index_patterns` to match if the exporter runs
with a different `storage.elasticsearch.index_prefix`. Changing the mapping only affects indices
created afterwards; the daily index rolls over at midnight UTC.

GET {{base_url}}/catalog
HTTP 200
[Captures]
catalog_etag: header "ETag"

GET {{base_url}}/catalog
If-None-Match: {{catalog_etag}}
HTTP 304

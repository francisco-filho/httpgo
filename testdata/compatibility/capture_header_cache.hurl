GET http://example.invalid/catalog
HTTP 200
[Captures]
etag: header "eTaG"
GET http://example.invalid/catalog
If-None-Match: {{etag}}
HTTP 304

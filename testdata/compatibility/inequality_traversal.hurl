GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$.x.y" != null
GET http://example.invalid/health

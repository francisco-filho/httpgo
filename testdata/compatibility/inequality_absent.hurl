GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$.missing" != null
GET http://example.invalid/health

GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$.missing" isInteger
GET http://example.invalid/health
HTTP 200

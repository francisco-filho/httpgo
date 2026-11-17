GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$.missing" isBoolean
GET http://example.invalid/health
HTTP 200

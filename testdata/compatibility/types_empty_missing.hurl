GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$.missing" isEmpty
GET http://example.invalid/health
HTTP 200

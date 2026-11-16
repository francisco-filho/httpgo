GET http://example.invalid/capture
HTTP *
[Asserts]
jsonpath "$.items" count == 0
GET http://example.invalid/health
HTTP 200

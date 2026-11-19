GET http://example.invalid/capture
HTTP *
[Asserts]
jsonpath "$.id" exists
GET http://example.invalid/health
HTTP 200

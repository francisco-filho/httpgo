GET http://example.invalid/capture
HTTP *
[Asserts]
jsonpath "$" count == 1
GET http://example.invalid/health
HTTP 200

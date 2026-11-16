GET http://example.invalid/capture
HTTP *
[Asserts]
jsonpath "$" count == 3
GET http://example.invalid/health
HTTP 200

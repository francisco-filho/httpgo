GET http://example.invalid/capture
HTTP *
[Asserts]
jsonpath "$" count == 0.5
GET http://example.invalid/health
HTTP 200

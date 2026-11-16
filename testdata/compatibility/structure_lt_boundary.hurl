GET http://example.invalid/capture
HTTP *
[Asserts]
jsonpath "$" < -1.25
GET http://example.invalid/health
HTTP 200

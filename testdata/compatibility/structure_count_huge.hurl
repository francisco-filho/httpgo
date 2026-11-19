GET http://example.invalid/capture
HTTP *
[Asserts]
jsonpath "$" count < 999999999999999999999999999999
GET http://example.invalid/health
HTTP 200

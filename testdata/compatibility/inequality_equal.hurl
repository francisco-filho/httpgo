GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$" != 1
GET http://example.invalid/health

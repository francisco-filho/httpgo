GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$" not exists
GET http://example.invalid/health
HTTP 200

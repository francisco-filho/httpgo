GET http://example.invalid/health
GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$" count != 0
GET http://example.invalid/health

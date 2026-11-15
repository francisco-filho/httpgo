GET http://example.invalid/health
HTTP 200
GET http://example.invalid/capture
[Asserts]
jsonpath "$" == null

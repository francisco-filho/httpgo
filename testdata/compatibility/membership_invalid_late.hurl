GET http://example.invalid/health
HTTP 200
GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$" not exists null
GET http://example.invalid/health
HTTP 200

GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$" not contains ""
jsonpath "$" not contains null
GET http://example.invalid/health
HTTP 200

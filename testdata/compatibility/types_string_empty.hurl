GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$" isString
GET http://example.invalid/health
HTTP 200

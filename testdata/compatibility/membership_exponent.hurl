GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$" contains 1e9999999999999999999999
GET http://example.invalid/health
HTTP 200

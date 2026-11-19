GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$" contains true
jsonpath "$" contains null
jsonpath "$" contains 1
jsonpath "$" not contains "1"
GET http://example.invalid/health
HTTP 200

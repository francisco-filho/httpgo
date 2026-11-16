GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$" contains 1.0
jsonpath "$" contains -0.0
jsonpath "$" contains 123.0
GET http://example.invalid/health
HTTP 200

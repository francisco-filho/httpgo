GET http://example.invalid/capture
HTTP *
[Asserts]
jsonpath "$[*].x" contains 1
GET http://example.invalid/health

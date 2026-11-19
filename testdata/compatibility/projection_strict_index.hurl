GET http://example.invalid/capture
HTTP *
[Asserts]
jsonpath "$[*][0]" contains 1
GET http://example.invalid/health

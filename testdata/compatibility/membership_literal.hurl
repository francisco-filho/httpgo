GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$" contains "{{name}}"
GET http://example.invalid/health
HTTP 200

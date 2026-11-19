GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$" matches "^{{pattern}}$"
GET http://example.invalid/health
HTTP 200

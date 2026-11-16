GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$" contains "Pizza"
GET http://example.invalid/health
HTTP 200

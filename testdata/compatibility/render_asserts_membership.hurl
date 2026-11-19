GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$" contains {{n}}
GET http://example.invalid/health
HTTP 200

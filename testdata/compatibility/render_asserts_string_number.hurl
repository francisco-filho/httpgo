GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$" == "{{n}}"
GET http://example.invalid/health
HTTP 200

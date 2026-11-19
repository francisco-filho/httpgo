GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$[{{index}}].id" == 1

GET http://example.invalid/health
HTTP 200

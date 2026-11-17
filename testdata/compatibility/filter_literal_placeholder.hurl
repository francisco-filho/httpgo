GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$[?(@.v=='{{static}}')].id" == "selected"

GET http://example.invalid/health
HTTP 200

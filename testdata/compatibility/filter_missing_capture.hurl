GET http://example.invalid/capture
HTTP 200
[Captures]
id: jsonpath "$[?(@.x==1)].id"

GET http://example.invalid/health
HTTP 200

GET http://example.invalid/capture
HTTP 200
[Captures]
id: jsonpath "$[?(@.x==1)].v"

POST http://example.invalid/health
{{id}}
HTTP 200

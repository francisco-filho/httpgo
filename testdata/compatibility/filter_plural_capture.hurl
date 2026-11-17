GET http://example.invalid/capture
HTTP 200
[Captures]
ids: jsonpath "$[?(@.x==1)].id"

POST http://example.invalid/health
{{ids}}
HTTP 200

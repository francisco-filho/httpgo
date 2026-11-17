GET http://example.invalid/capture
HTTP 200
[Captures]
baseline: jsonpath "$[?(@.ok==true)].items" count
POST http://example.invalid/health
Content-Type: application/json
{"count":{{baseline}}}
HTTP 200

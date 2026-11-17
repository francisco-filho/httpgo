GET http://example.invalid/capture
HTTP *
[Captures]
values: jsonpath "$[*]"
POST http://example.invalid/health
{{values}}
HTTP 200

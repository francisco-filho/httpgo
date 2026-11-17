GET http://example.invalid/capture
HTTP *
[Captures]
values: jsonpath "$[*]"
GET http://example.invalid/health

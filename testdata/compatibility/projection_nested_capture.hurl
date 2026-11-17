GET http://example.invalid/capture
HTTP *
[Captures]
values: jsonpath "$.categories[*].products[*].id"
POST http://example.invalid/health
{{values}}
HTTP 200

GET http://example.invalid/capture
HTTP 200
[Captures]
order_number: jsonpath "$.number"

GET http://example.invalid/capture
HTTP 200
[Captures]
selected: jsonpath "$.items[?(@.number=={{order_number}})].id"

POST http://example.invalid/health
{{selected}}
HTTP 200

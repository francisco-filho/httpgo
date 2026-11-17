POST http://example.invalid/capture
HTTP 200
[Captures]
order_number: jsonpath "$.number"
order_name: jsonpath "$.name"
GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$.number" == {{order_number}}
jsonpath "$.name" == "{{order_name}}"
GET http://example.invalid/health
HTTP 200

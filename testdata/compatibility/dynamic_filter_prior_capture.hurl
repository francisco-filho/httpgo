GET http://example.invalid/capture
HTTP 200
[Captures]
order_number: jsonpath "$.number"

GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$.items[?(@.number=={{order_number}})].id" == "selected"

GET http://example.invalid/health
HTTP 200

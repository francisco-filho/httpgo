GET http://example.invalid/capture
HTTP 200
[Captures]
number: jsonpath "$.number"

GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$.items[?(@.number=={{number}})].status" == "PLACED"

GET http://example.invalid/health
HTTP 200

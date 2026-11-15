GET http://example.invalid/capture
HTTP 200
[Captures]
x: jsonpath "$.missing"
[Asserts]
jsonpath "$.value" == 2
GET http://example.invalid/health
HTTP 200

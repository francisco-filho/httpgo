GET http://example.invalid/capture
HTTP 200
[Captures]
value: jsonpath "$.missing"
[Asserts]
header "X-Test" == "expected"
GET http://example.invalid/health
HTTP 200

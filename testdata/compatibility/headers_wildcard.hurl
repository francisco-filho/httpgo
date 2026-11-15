GET http://example.invalid/health
HTTP *
[Asserts]
header "X-Test" exists
GET http://example.invalid/missing
HTTP *
[Asserts]
header "X-Test" contains "alu"
GET http://example.invalid/unavailable
HTTP *
[Asserts]
header "X-Test" == "value"

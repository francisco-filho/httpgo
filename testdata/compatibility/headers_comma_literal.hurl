GET http://example.invalid/health
HTTP *
[Asserts]
header "X-Test" == "first,second"
header "X-Test" contains "st,se"
GET http://example.invalid/missing
HTTP 404

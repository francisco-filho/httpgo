GET http://example.invalid/capture
HTTP 200
[Captures]
x: header "Absent"
[Asserts]
header "X-Test" == "different"
GET http://example.invalid/health

GET http://example.invalid/capture
HTTP 200
[Asserts]
header "X" contains "{{needle}}"
body contains "{{needle}}"
GET http://example.invalid/health
HTTP 200

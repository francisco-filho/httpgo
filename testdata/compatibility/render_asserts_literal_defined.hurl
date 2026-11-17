GET http://example.invalid/capture
HTTP 200
[Asserts]
header "X" == "{{needle}}"
body contains "{{needle}}"
jsonpath "$" == "{{needle}}"
GET http://example.invalid/health
HTTP 200

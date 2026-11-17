GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$.id" == "{{id}}"
header "X-Id" == "{{id}}"
[Captures]
id: jsonpath "$.id"
GET http://example.invalid/health
X-Probe: {{id}}
HTTP 200

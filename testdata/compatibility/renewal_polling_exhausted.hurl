POST http://example.invalid/poll
X-Probe: {{id}}
Content-Type: application/json
[Options]
retry: 2
retry-interval: 0
{"id":"{{id}}"}
HTTP 200
[Captures]
id: jsonpath "$.id"
[Asserts]
jsonpath "$.id" == "{{id}}"
jsonpath "$.status" == "READY"
GET http://example.invalid/health
X-Probe: {{id}}
HTTP 200

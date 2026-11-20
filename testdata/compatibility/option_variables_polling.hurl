POST http://example.invalid/poll
Content-Type: application/json
X-Probe: {{id}}
[Options]
variable: id=assigned
retry: 2
retry-interval: 0
{"id":"{{id}}"}
HTTP 200
[Captures]
id: jsonpath "$.id"
[Asserts]
jsonpath "$.ready" == true
GET http://example.invalid/health
X-Probe: {{id}}
HTTP 200

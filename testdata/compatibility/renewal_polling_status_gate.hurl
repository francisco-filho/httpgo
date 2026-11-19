GET http://example.invalid/poll
X-Probe: {{id}}
[Options]
retry: 2
retry-interval: 0
HTTP 200
[Captures]
id: jsonpath "$.id"
GET http://example.invalid/health
X-Probe: {{id}}
HTTP 200

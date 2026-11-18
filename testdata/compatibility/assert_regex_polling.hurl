GET http://example.invalid/poll
X-Probe: {{id}}
[Options]
retry: 1
retry-interval: 0
HTTP 200
[Captures]
id: header "X-ID"
[Asserts]
regex "id=([^;]+)" == "{{id}}"
regex "state=(.*)" == "READY"
GET http://example.invalid/health
X-Probe: {{id}}
HTTP 200

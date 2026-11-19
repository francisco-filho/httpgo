GET http://example.invalid/capture
HTTP 200
[Captures]
id: jsonpath "$.first"
id: jsonpath "$.second"
GET http://example.invalid/health
X-Probe: {{id}}
HTTP 200

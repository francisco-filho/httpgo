POST http://example.invalid/capture
Content-Type: application/json
X-Probe: {{id}}
[Options]
variable: id=discarded
variable: id=assigned
variable: count=3
{"id":"{{id}}","count":{{count}}}
HTTP 200
[Captures]
id: jsonpath "$.id"
GET http://example.invalid/{{id}}
X-Probe: {{id}}
HTTP 200
GET http://example.invalid/{{id}}
X-Probe: {{id}}
[Options]
variable: id=final
HTTP 200
GET http://example.invalid/{{id}}
X-Probe: {{id}}
HTTP 200

GET http://example.invalid/capture
HTTP 200
[Captures]
id: jsonpath "$.id"
selected: jsonpath "$.items[?(@.id=='{{id}}')].values" count
GET http://example.invalid/health
X-Probe: {{selected}}
HTTP 200

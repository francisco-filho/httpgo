GET {{base}}/capture
HTTP 200
[Captures]
id: jsonpath "$.id"
POST {{base}}/products/{{id}}
Content-Type: application/json
X-Probe: {{id}}
{"id":{{id}}}
HTTP 200

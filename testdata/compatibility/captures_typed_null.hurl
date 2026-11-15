GET {{base}}/capture
HTTP 200
[Captures]
value: jsonpath "$"
POST {{base}}/{{value}}
X-Probe: pre{{value}}
Content-Type: application/json
{"value":{{value}},"text":"pre{{value}}"}
HTTP 200

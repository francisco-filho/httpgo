GET {{base}}/capture
HTTP 200
[Captures]
value: jsonpath "$"
POST {{base}}/health
Content-Type: application/json
{{value}}
HTTP 200

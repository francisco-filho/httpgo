GET {{base}}/capture
HTTP 200
[Captures]
token: jsonpath "$"
POST {{base}}/health
Content-Type: application/json
{"value":{{token}},"text":"{{token}}"}

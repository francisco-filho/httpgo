GET {{base}}/capture
HTTP 200
[Captures]
value: jsonpath "$"

POST {{base}}/echo
Content-Type: application/json
{{value}}
HTTP 200

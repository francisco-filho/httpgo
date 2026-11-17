GET {{base_url}}/first
GET {{base_url}}/late
HTTP *
[Captures]
x: jsonpath "$[?(@.id == 1)]"

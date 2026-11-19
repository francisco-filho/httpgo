GET {{base_url}}/first
GET {{base_url}}/late
HTTP 200
[Captures]
x: jsonpath "$.items[{{index}}]"

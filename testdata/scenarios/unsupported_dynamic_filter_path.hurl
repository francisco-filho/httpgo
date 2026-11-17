GET {{base_url}}/first
GET {{base_url}}/orders
HTTP 200
[Captures]
selected: jsonpath "{{path}}"

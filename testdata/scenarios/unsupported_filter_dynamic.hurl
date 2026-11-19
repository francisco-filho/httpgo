GET {{base_url}}/first
GET {{base_url}}/products
HTTP 200
[Captures]
id: jsonpath "$[?(@.x == {{value}})].id"

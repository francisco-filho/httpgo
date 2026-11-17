GET {{base_url}}/first
GET {{base_url}}/products
HTTP 200
[Asserts]
jsonpath "$[?(@.x == 1 && @.y == true)].id" exists

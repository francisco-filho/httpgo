GET {{base_url}}/products
HTTP 200
[Captures]
pizza_id: jsonpath "$[?(@.slug=='pizza')].id"
[Asserts]
jsonpath "$[?(@.slug=='pizza')].active" == true

GET {{base_url}}/products/{{pizza_id}}
HTTP 200

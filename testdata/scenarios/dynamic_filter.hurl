GET {{base_url}}/first
HTTP 200
[Captures]
order_number: jsonpath "$.number"

GET {{base_url}}/orders
HTTP 200
[Asserts]
jsonpath "$.items[?(@.number=={{order_number}})].status" == "PLACED"
[Captures]
selected_id: jsonpath "$.items[?(@.number=={{order_number}})].id"

GET {{base_url}}/{{selected_id}}
HTTP 200

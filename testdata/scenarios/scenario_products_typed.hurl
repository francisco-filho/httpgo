POST {{api.baseUrl}}/products
Content-Type: application/json
{"name":"{{product.name}}","price":{{product.price}},"active":{{product.active}},"code":{{product.code}}}

HTTP 201

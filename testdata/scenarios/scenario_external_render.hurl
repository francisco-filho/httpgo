# Extensão HTTPGo: executar com --render-files e as três variáveis product.*
POST http://localhost:8080/products
Content-Type: application/json
file,../payloads/product_template.json;
HTTP 201

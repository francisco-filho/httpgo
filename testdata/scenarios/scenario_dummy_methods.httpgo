# Servidor demonstrativo: make serve (sem persistência de produtos).
POST http://localhost:8080/products
Content-Type: application/json
{"name":"Pizza","price":4500}
HTTP 201

PUT http://localhost:8080/products/1
Content-Type: application/json
{"name":"Pizza grande","price":5000}
HTTP 200

PATCH http://localhost:8080/products/1
Content-Type: application/json
{"price":4900}
HTTP 200

POST http://localhost:8080/echo
Content-Type: application/json
{
  "values": [true, null, 42],
  "text": "HTTP 200 # não é um delimitador aqui"
}
HTTP 200

DELETE http://localhost:8080/products/1
HTTP 204

GET http://localhost:8080/health
HTTP 200

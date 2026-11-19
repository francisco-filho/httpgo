# Extensão HTTPGo: --render-files rejeita template malformado na segunda entrada
GET http://localhost:8080/health
HTTP 200
POST http://localhost:8080/products
file,../payloads/external_render_malformed.json;
HTTP 201
GET http://localhost:8080/health
HTTP 200

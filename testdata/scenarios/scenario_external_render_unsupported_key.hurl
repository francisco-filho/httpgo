# Extensão HTTPGo: --render-files rejeita chave dinâmica somente nesta entrada
GET http://localhost:8080/health
HTTP 200
POST http://localhost:8080/products
file,../payloads/external_render_dynamic_key.json;
HTTP 201
GET http://localhost:8080/health
HTTP 200

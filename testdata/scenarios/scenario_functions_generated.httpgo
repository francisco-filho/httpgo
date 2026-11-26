# Gera carimbo de tempo e identificadores únicos a cada execução.
# Execute contra uma API local que aceite o payload e responda 201.
POST {{api.baseUrl}}/products
Content-Type: application/json
{"requestId":"{{newUuid}}","createdAt":"{{newDate}}","label":"pedido-{{newUuid}}"}
HTTP 201
